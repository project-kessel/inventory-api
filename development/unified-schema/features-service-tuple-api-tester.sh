#!/bin/bash
#
# End-to-end tester for the features service model (JSON and unified schema),
# driven through the tuple API, against a running inventory-api instance. For
# the same model driven through ReportResource and the relationship delta
# endpoints, see features-service-schema-tester.sh.
#
# Relationships are written through the tuple-layer API (KesselTupleService's
# CreateTuples/DeleteTuples), which writes straight to SpiceDB without going
# through ReportResource or the consumer. No inventory resources are created —
# what this exercises is the schema itself: the features namespace and the
# features-reporter relations folded onto rbac/workspace.
#
# Two naming conventions appear below for the same relationships, and the
# server translates between them (see internal/biz/model/schema_translation.go):
#
#   kessel names — used in every grpcurl payload (CreateTuples, DeleteTuples,
#     Check). The namespace is the reporter and the relation is the one the
#     resource's reporter schema declares, e.g.
#     features/workspace#direct_service_preferences and
#     features/billing_account#services.
#
#   serialized names — what lands in SpiceDB, and therefore what the zed seeds
#     and the tuple assertions use. The features workspace is a derived type:
#     it has no type of its own in SpiceDB, so it is folded onto rbac/workspace
#     with the relations it owns prefixed by features_workspace_, and every
#     stored relation picks up a t_ prefix. The two examples above are stored
#     as rbac/workspace#t_features_workspace_direct_service_preferences and
#     features/billing_account#t_services; the enabled_services permission is
#     evaluated as rbac/workspace#features_workspace_enabled_services.
#
# Because the writes bypass inventory, the Checks cannot rely on the server's
# at_least_as_acknowledged default to pin them to a fresh revision; each Check
# instead passes the consistency token returned by the preceding write. See
# the CONSISTENCY_TOKEN comment below for why this is required.
#
# Prerequisites:
#   - Full Kessel stack running in unified schema mode (via
#     `make kessel-up-unified` or an ephemeral deployment), so the features
#     service schema is loaded into SpiceDB
#   - `zed` on PATH and configured (`zed context set`) against the same
#     SpiceDB instance inventory-api writes to
#   - `grpcurl` on PATH for the API calls
#
# What this script does, in order:
#   1. Seeds a workspace hierarchy directly into SpiceDB via zed
#   2. Seeds billing account and workspace relationships via CreateTuples
#   3. Verifies the enabled_services permission with Check calls
#   4. Asserts the expected SpiceDB tuples with targeted PASS/FAIL checks
#   5. Exercises CreateTuples deltas, then re-verifies Check + tuples
#   6. Exercises DeleteTuples, then re-verifies Check + tuples
#      (the deletes reverse the creates, returning SpiceDB to its earlier state)
#
# Access model being tested:
#   A workspace enables a service (enabled_services = allowed) only when BOTH:
#     - the workspace, or one of its ancestors, desires the service
#       (direct_service_preferences, inherited down the t_parent tree), AND
#     - a billing account linked to the workspace, or one of its ancestors,
#       pays for the service (services on the account)
#   Desire and payment each propagate independently down the hierarchy, so
#   the two sides can be satisfied at different levels of the tree. Most of
#   the checks below exercise one side being satisfied and the other not.

PORT_NUMBER=$1

if [[ -z "$PORT_NUMBER" ]]; then
  echo "Missing port number"
  echo "./features-service-tuple-api-tester.sh PORT_NUMBER"
  exit 1
fi

# ---------------------------------------------------------------------------
# Skip the zed CLI's version check against the server: the only thing it
# produces in this script is the "SpiceDB being called is out of date"
# warning, which just adds noise. (Also settable per-flag via
# `--skip-version-check`; the env var applies to every zed call, including
# manual ones run in this shell afterwards.)
# ---------------------------------------------------------------------------
export ZED_SKIP_VERSION_CHECK=true

# ---------------------------------------------------------------------------
# PASS/FAIL assertion helpers. Instead of dumping every relationship in
# SpiceDB (noisy and hard to scan), each expected tuple is verified with a
# targeted `zed relationship read` and reported as a single PASS/FAIL line.
# A running tally is kept and printed by print_assert_summary.
#
# zed talks to SpiceDB directly and sees no translation, so the RESOURCE and
# RELATION arguments below are serialized names (rbac/workspace,
# t_features_workspace_*, features/billing_account, t_services) rather than
# the kessel names the grpcurl payloads use.
#
# The reads pass --consistency-full so they are evaluated at the newest
# revision in the database. Without it SpiceDB may answer from its quantized
# snapshot, which can still be behind the writes this script just made.
#
# To eyeball the raw tuples manually while debugging a FAIL, run:
#   zed relationship read --consistency-full features/billing_account
#   zed relationship read --consistency-full rbac/workspace
# ---------------------------------------------------------------------------

PASS_COUNT=0
FAIL_COUNT=0

# ---------------------------------------------------------------------------
# CONSISTENCY_TOKEN holds the ConsistencyToken returned by the most recent
# CreateTuples/DeleteTuples call, and every Check below passes it as
# at_least_as_fresh.
#
# This is required, not an optimization. CreateTuples/DeleteTuples write to
# SpiceDB synchronously, but SpiceDB serves reads from a quantized snapshot
# that can lag the latest revision by several seconds. A Check that does not
# name a revision is answered from that possibly-stale snapshot, so it can be
# evaluated against state from before the write and fail spuriously.
#
# The server-side at_least_as_acknowledged default does not help here: it
# resolves a token by looking the resource up in the inventory database, and
# this script never creates inventory resources (it writes tuples directly),
# so that lookup misses and falls back to minimize_latency. Carrying the token
# forward from the write is the only way to pin the read to the right revision.
#
# SpiceDB revisions are monotonic, so the token from the most recent write
# also covers every earlier write, including the zed-seeded t_parent tuples.
#
# Each CreateTuples/DeleteTuples call below therefore pipes its response
# through sed to pull out the token and assign it here. grpcurl errors go to
# stderr and so are still printed; a failed call leaves the token empty, which
# assert_check reports rather than silently checking against stale state.
# ---------------------------------------------------------------------------
CONSISTENCY_TOKEN=""

# assert_tuple_exists RESOURCE RELATION SUBJECT_ID DESCRIPTION
#   Checks that the given relation on the resource has a tuple whose subject
#   matches SUBJECT_ID.
#   PASS = the expected tuple exists in SpiceDB.
#   FAIL = the expected tuple is missing; the tuples that ARE present are
#   printed to help pinpoint what went wrong.
assert_tuple_exists() {
  local actual
  actual=$(zed relationship read --consistency-full "$1" "$2" 2>/dev/null)
  if echo "$actual" | grep -q "$3"; then
    echo "PASS: $4 — tuple exists, as expected"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "FAIL: $4 — expected tuple is MISSING"
    echo "      expected: $1 $2 -> subject $3"
    if [[ -z "$actual" ]]; then
      echo "      actual:   no tuples found for $1 $2"
    else
      echo "      actual tuples on $1 $2:"
      echo "$actual" | sed 's/^/        /'
    fi
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# assert_tuple_absent RESOURCE RELATION SUBJECT_ID DESCRIPTION
#   Checks that the given relation on the resource has NO tuple whose subject
#   matches SUBJECT_ID.
#   PASS = the tuple is absent, as expected.
#   FAIL = the tuple exists but should not; the offending tuples are printed.
#   FAIL = the zed read itself failed — a failed read is never treated as
#   proof of absence.
assert_tuple_absent() {
  local actual
  if ! actual=$(zed relationship read --consistency-full "$1" "$2" 2>/dev/null); then
    echo "FAIL: $4 — zed relationship read failed"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    return
  fi
  if echo "$actual" | grep -q "$3"; then
    echo "FAIL: $4 — tuple EXISTS but was not expected"
    echo "      unexpected: $1 $2 -> subject $3"
    echo "      offending tuples on $1 $2:"
    echo "$actual" | grep "$3" | sed 's/^/        /'
    FAIL_COUNT=$((FAIL_COUNT + 1))
  else
    echo "PASS: $4 — no such tuple, as expected"
    PASS_COUNT=$((PASS_COUNT + 1))
  fi
}

# assert_check EXPECTED_ALLOWED DESCRIPTION
#   Reads a Check request (JSON) from stdin, invokes the Check endpoint, and
#   asserts that the response's "allowed" enum equals the expectation
#   ("true" or "false"). grpcurl renders the proto Allowed enum as its name
#   string (ALLOWED_TRUE / ALLOWED_FALSE), so the expectation is mapped to
#   the enum name before matching. PASS/FAIL is recorded like the tuple
#   assertions; a failed grpcurl call is recorded as a failure rather than
#   a pass.
#
#   The request is expected to carry "${CONSISTENCY_TOKEN}" as its
#   at_least_as_fresh token, so a missing token is caught here rather than
#   surfacing as an opaque request-validation error from the server.
assert_check() {
  local expected="$1" description="$2" expected_enum
  case "$expected" in
    true)  expected_enum="ALLOWED_TRUE" ;;
    false) expected_enum="ALLOWED_FALSE" ;;
    *)     echo "FAIL: $description — invalid expectation '$expected'"; FAIL_COUNT=$((FAIL_COUNT + 1)); return ;;
  esac
  if [[ -z "$CONSISTENCY_TOKEN" ]]; then
    cat >/dev/null
    echo "FAIL: $description — no consistency token captured from a prior CreateTuples/DeleteTuples call"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    return
  fi
  local response
  if ! response=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.Check 2>&1); then
    echo "FAIL: $description — Check call failed"
    echo "      error:"
    echo "$response" | sed 's/^/        /'
    FAIL_COUNT=$((FAIL_COUNT + 1))
    return
  fi
  if echo "$response" | grep -Eq "\"allowed\": *\"${expected_enum}\""; then
    echo "PASS: $description — allowed=${expected_enum}, as expected"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "FAIL: $description — expected allowed=${expected_enum}"
    echo "      response:"
    echo "$response" | sed 's/^/        /'
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
}

# print_assert_summary prints the running PASS/FAIL tally.
print_assert_summary() {
  echo ""
  echo "========================================================"
  echo "Results: ${PASS_COUNT} passed, ${FAIL_COUNT} failed"
  echo "========================================================"
}

### REFERENCE INFO
# Test Workspaces Info
# root           31a6d1a6-b395-11f1-adbd-6ae246604903
# default        8456f286-b396-11f1-9527-6ae246604903
# department a   8adc4a2a-b396-11f1-8aa5-6ae246604903
# department b   9015d2ae-b396-11f1-9308-6ae246604903
# team 1         9554d7a6-b396-11f1-8fa2-6ae246604903
# team 2         9a4cd31c-b396-11f1-af2e-6ae246604903
# team 3         a0af6abc-b396-11f1-ae2b-6ae246604903

# Test Services info
#service a          4f829a2c-b398-11f1-ba8e-6ae246604903
#service b          54c3d5a0-b398-11f1-9332-6ae246604903
#service c          59be7c18-b398-11f1-9781-6ae246604903
#service d          5e118fc6-b398-11f1-857a-6ae246604903

# Test Accounts Info
#main account       3055cc32-b398-11f1-bdd0-6ae246604903
#special account    3ba27996-b398-11f1-bf17-6ae246604903

# The workspace hierarchy is seeded directly into SpiceDB with zed rather than
# through the inventory API: the t_parent tuples form the org tree that both
# "desire" (direct_service_preferences) and "payment" (billing account links)
# propagate down when SpiceDB evaluates enabled_services.
#
# Being zed calls, these use serialized names: t_parent belongs to
# rbac/workspace itself rather than to the features reporter, so it carries no
# features_workspace_ prefix.
#
# Hierarchy being created:
#
#            root
#              |
#           default
#          /        \
#      dept a      dept b
#        |         /     \
#     team 1    team 2   team 3
#
echo "Creating base relationships to workspaces..."
zed relationship create rbac/workspace:8456f286-b396-11f1-9527-6ae246604903 t_parent rbac/workspace:31a6d1a6-b395-11f1-adbd-6ae246604903
zed relationship create rbac/workspace:8adc4a2a-b396-11f1-8aa5-6ae246604903 t_parent rbac/workspace:8456f286-b396-11f1-9527-6ae246604903  # dept a to default
zed relationship create rbac/workspace:9015d2ae-b396-11f1-9308-6ae246604903 t_parent rbac/workspace:8456f286-b396-11f1-9527-6ae246604903  # dept b to default
zed relationship create rbac/workspace:9554d7a6-b396-11f1-8fa2-6ae246604903 t_parent rbac/workspace:8adc4a2a-b396-11f1-8aa5-6ae246604903  # team 1 to dept a
zed relationship create rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_parent rbac/workspace:9015d2ae-b396-11f1-9308-6ae246604903  # team 2 to dept b
zed relationship create rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_parent rbac/workspace:9015d2ae-b396-11f1-9308-6ae246604903  # team 3 to dept b


echo ""
echo "================================================================="
echo "Testing CreateTuples for Billing Accounts and Features Workspaces"
echo "================================================================="

# Each CreateTuples below writes the tuples for one resource straight into
# SpiceDB. The payloads use kessel names — features/billing_account with
# relation "services", features/workspace with "direct_billing_account" and
# "direct_service_preferences" — which the server rewrites into the serialized
# names the assertions further below read back with zed.
#
# The seeded state sets up both sides of the access model:
#   payment:  main account pays for services a+b, special account pays for c
#   desire:   root wants a, dept a wants b, dept b links the special account,
#             team 1 wants d, team 3 wants c (team 2 wants nothing)
echo "Main account pays for: service a, service b"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "billing_account"},
          "id": "3055cc32-b398-11f1-bdd0-6ae246604903"
        },
        "relation": "services",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "4f829a2c-b398-11f1-ba8e-6ae246604903"
          }
        }
      },
      {
        "resource": {
          "type": {"namespace": "features", "name": "billing_account"},
          "id": "3055cc32-b398-11f1-bdd0-6ae246604903"
        },
        "relation": "services",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "54c3d5a0-b398-11f1-9332-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "special account pays for: service c"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "billing_account"},
          "id": "3ba27996-b398-11f1-bf17-6ae246604903"
        },
        "relation": "services",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "59be7c18-b398-11f1-9781-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "root workspace uses main account, subscribes service a"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "31a6d1a6-b395-11f1-adbd-6ae246604903"
        },
        "relation": "direct_billing_account",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "billing_account"},
            "id": "3055cc32-b398-11f1-bdd0-6ae246604903"
          }
        }
      },
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "31a6d1a6-b395-11f1-adbd-6ae246604903"
        },
        "relation": "direct_service_preferences",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "4f829a2c-b398-11f1-ba8e-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "dept a subscribes service b"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "8adc4a2a-b396-11f1-8aa5-6ae246604903"
        },
        "relation": "direct_service_preferences",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "54c3d5a0-b398-11f1-9332-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "dept b uses special account"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "9015d2ae-b396-11f1-9308-6ae246604903"
        },
        "relation": "direct_billing_account",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "billing_account"},
            "id": "3ba27996-b398-11f1-bf17-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "team 1 subscribes service d"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "9554d7a6-b396-11f1-8fa2-6ae246604903"
        },
        "relation": "direct_service_preferences",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "5e118fc6-b398-11f1-857a-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "team 3 subscribes service c"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "a0af6abc-b396-11f1-ae2b-6ae246604903"
        },
        "relation": "direct_service_preferences",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "59be7c18-b398-11f1-9781-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

# No sleep is needed before the Checks: each one pins itself to the revision
# of the last write via CONSISTENCY_TOKEN rather than waiting for SpiceDB's
# quantized snapshot to catch up. The zed reads further below are pinned the
# same way, via --consistency-full in the assert_tuple_* helpers.
echo "All tuples written...time for Checks"

# Each Check below evaluates the enabled_services PERMISSION and is asserted
# by assert_check, which reads the request from the heredoc, invokes the
# endpoint, and verifies the response's "allowed" value against the expected
# outcome — recording PASS/FAIL like the tuple assertions.
#
# Each request names the kessel permission, features/workspace#enabled_services;
# the server evaluates rbac/workspace#features_workspace_enabled_services.
#
# Allowed: root desires service a and root's main account pays for it; both
# sides inherit down the tree to team 1.
assert_check true "Team 1 should have access to service A" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "4f829a2c-b398-11f1-ba8e-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

# Allowed: dept a (team 1's ancestor) desires service b and the main account
# pays for it.
assert_check true "Team 1 should also have access to service B" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "54c3d5a0-b398-11f1-9332-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

# DENIED: team 1 desires service d directly, but no billing account pays for
# service d. Desire without payment is not enough.
assert_check false "Team 1 should NOT have access to service D" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "5e118fc6-b398-11f1-857a-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

# DENIED: dept b's special account pays for service c, but team 2 does not
# desire it (only team 3 does). Payment without desire is not enough.
assert_check false "Team 2 should NOT have access to service C" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9a4cd31c-b396-11f1-af2e-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "59be7c18-b398-11f1-9781-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

# Allowed: team 3 desires service c directly AND dept b's special account
# pays for it — both sides satisfied.
assert_check true "Team 3 SHOULD have access to service C" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "a0af6abc-b396-11f1-ae2b-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "59be7c18-b398-11f1-9781-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

# ---------------------------------------------------------------------------
# Tuple assertions: verify the state the CreateTuples calls produced, one
# PASS/FAIL line per expected tuple. They read SpiceDB through zed, so they
# name the serialized form of what the payloads above wrote: payment as
# features/billing_account#t_services, and desire plus the billing link as
# rbac/workspace#t_features_workspace_* — along with the zed-seeded t_parent
# hierarchy.
# ---------------------------------------------------------------------------

echo "Validating the seeded relationships landed in SpiceDB..."

echo "-- Payment: billing accounts --"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 4f829a2c-b398-11f1-ba8e-6ae246604903 "main account pays for service a"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 54c3d5a0-b398-11f1-9332-6ae246604903 "main account pays for service b"
assert_tuple_exists features/billing_account:3ba27996-b398-11f1-bf17-6ae246604903 t_services 59be7c18-b398-11f1-9781-6ae246604903 "special account pays for service c"
assert_tuple_absent features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 5e118fc6-b398-11f1-857a-6ae246604903 "nobody pays for service d yet (why Team 1's check was denied)"

echo "-- Hierarchy: zed-seeded t_parent links --"
assert_tuple_exists rbac/workspace:8456f286-b396-11f1-9527-6ae246604903 t_parent 31a6d1a6-b395-11f1-adbd-6ae246604903 "default's parent is root"
assert_tuple_exists rbac/workspace:8adc4a2a-b396-11f1-8aa5-6ae246604903 t_parent 8456f286-b396-11f1-9527-6ae246604903 "dept a's parent is default"
assert_tuple_exists rbac/workspace:9015d2ae-b396-11f1-9308-6ae246604903 t_parent 8456f286-b396-11f1-9527-6ae246604903 "dept b's parent is default"
assert_tuple_exists rbac/workspace:9554d7a6-b396-11f1-8fa2-6ae246604903 t_parent 8adc4a2a-b396-11f1-8aa5-6ae246604903 "team 1's parent is dept a"
assert_tuple_exists rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_parent 9015d2ae-b396-11f1-9308-6ae246604903 "team 2's parent is dept b"
assert_tuple_exists rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_parent 9015d2ae-b396-11f1-9308-6ae246604903 "team 3's parent is dept b"

echo "-- Desire: workspace preferences and billing links --"
assert_tuple_exists rbac/workspace:31a6d1a6-b395-11f1-adbd-6ae246604903 t_features_workspace_direct_billing_account 3055cc32-b398-11f1-bdd0-6ae246604903 "root is billed by the main account"
assert_tuple_exists rbac/workspace:31a6d1a6-b395-11f1-adbd-6ae246604903 t_features_workspace_direct_service_preferences 4f829a2c-b398-11f1-ba8e-6ae246604903 "root desires service a"
assert_tuple_exists rbac/workspace:8adc4a2a-b396-11f1-8aa5-6ae246604903 t_features_workspace_direct_service_preferences 54c3d5a0-b398-11f1-9332-6ae246604903 "dept a desires service b"
assert_tuple_exists rbac/workspace:9015d2ae-b396-11f1-9308-6ae246604903 t_features_workspace_direct_billing_account 3ba27996-b398-11f1-bf17-6ae246604903 "dept b is billed by the special account"
assert_tuple_exists rbac/workspace:9554d7a6-b396-11f1-8fa2-6ae246604903 t_features_workspace_direct_service_preferences 5e118fc6-b398-11f1-857a-6ae246604903 "team 1 desires service d"
assert_tuple_exists rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 3 desires service c"
assert_tuple_absent rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 2 desires nothing yet (why its check was denied)"


echo ""
echo "=============================================================="
echo "Testing CreateTuples as a delta (merging into existing tuples)"
echo "=============================================================="

# Each of the two creates below fixes the missing side of a check that was
# DENIED in the first round: the first adds the missing payment, the second
# the missing desire. Both must merge into the existing tuples rather than
# replacing them.
echo "Adding service D to the main billing account's paid services"
echo "(Team 1 already desires D directly; it was denied above because nobody paid for it)"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "billing_account"},
          "id": "3055cc32-b398-11f1-bdd0-6ae246604903"
        },
        "relation": "services",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "5e118fc6-b398-11f1-857a-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

echo "Adding service C as a direct preference on Team 2's workspace"
echo "(Dept B's special account already pays for C; Team 2 was denied above because it did not desire it)"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/CreateTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "upsert": true,
    "tuples": [
      {
        "resource": {
          "type": {"namespace": "features", "name": "workspace"},
          "id": "9a4cd31c-b396-11f1-af2e-6ae246604903"
        },
        "relation": "direct_service_preferences",
        "subject": {
          "subject": {
            "type": {"namespace": "features", "name": "service"},
            "id": "59be7c18-b398-11f1-9781-6ae246604903"
          }
        }
      }
    ]
}
EOF
)

# Both previously-DENIED checks should now flip to allowed, because each
# create satisfied the missing half of the desire+payment equation.

assert_check true "Team 1 SHOULD NOW have access to service D (desired directly + now paid by main account)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "5e118fc6-b398-11f1-857a-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

assert_check true "Team 2 SHOULD NOW have access to service C (now desired directly + paid by special account)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9a4cd31c-b396-11f1-af2e-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "59be7c18-b398-11f1-9781-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

echo "Validating the created relationships landed in SpiceDB..."

echo "-- Create: payment merge (service d onto the main account) --"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 5e118fc6-b398-11f1-857a-6ae246604903 "main account now pays for service d"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 4f829a2c-b398-11f1-ba8e-6ae246604903 "main account STILL pays for service a (Create merges, does not replace)"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 54c3d5a0-b398-11f1-9332-6ae246604903 "main account STILL pays for service b (Create merges, does not replace)"

echo "-- Create: desire (service c onto team 2) --"
assert_tuple_exists rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 2 now desires service c"

echo "-- Unrelated state must be untouched by the Creates --"
assert_tuple_exists rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 3 still desires service c (unrelated resource unchanged)"

echo ""
echo "========================================================"
echo "Testing the DeleteTuples endpoint"
echo "========================================================"

# The two deletes below reverse the creates from the previous section,
# returning SpiceDB to its earlier state. Each filter names a single subject,
# so only the targeted tuple is removed.
#
# Filters are written in kessel names as well, and a filter on a derived type
# (the features workspace) must name a relation: without one the rewrite onto
# rbac/workspace would match that workspace's other tuples too, so the server
# rejects the filter rather than deleting more than was asked.
echo "Removing service D from the main billing account's paid services"
echo "(This reverses the create above; Team 1 desires D but nobody pays for it again)"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/DeleteTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "filter": {
      "resourceNamespace": "features",
      "resourceType": "billing_account",
      "resourceId": "3055cc32-b398-11f1-bdd0-6ae246604903",
      "relation": "services",
      "subjectFilter": {
        "subjectNamespace": "features",
        "subjectType": "service",
        "subjectId": "5e118fc6-b398-11f1-857a-6ae246604903"
      }
    }
}
EOF
)

echo "Removing service C from Team 2's direct preferences"
echo "(This reverses the create above; Dept B's account still pays for C but Team 2 no longer desires it)"
CONSISTENCY_TOKEN=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselTupleService/DeleteTuples <<EOF | sed -n 's/.*"token": *"\([^"]*\)".*/\1/p'
{
    "filter": {
      "resourceNamespace": "features",
      "resourceType": "workspace",
      "resourceId": "9a4cd31c-b396-11f1-af2e-6ae246604903",
      "relation": "direct_service_preferences",
      "subjectFilter": {
        "subjectNamespace": "features",
        "subjectType": "service",
        "subjectId": "59be7c18-b398-11f1-9781-6ae246604903"
      }
    }
}
EOF
)

# The two checks that flipped to allowed after the creates should now flip
# back to denied, and the final check confirms an unrelated relationship was
# untouched.
assert_check false "Team 1 should NO LONGER have access to service D (no longer paid for by main account)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "5e118fc6-b398-11f1-857a-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

assert_check false "Team 2 should NO LONGER have access to service C (no longer desired directly)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9a4cd31c-b396-11f1-af2e-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "59be7c18-b398-11f1-9781-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

assert_check true "Team 1 should STILL have access to service A (unaffected by the removals)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
    "reporter": {
        "type": "features"
    }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "4f829a2c-b398-11f1-ba8e-6ae246604903",
      "reporter": {
        "type": "features"
      }
    }
  },
  "consistency": {
    "atLeastAsFresh": {
      "token": "${CONSISTENCY_TOKEN}"
    }
  }
}
EOF

echo "Validating the removed relationships are gone from SpiceDB..."

echo "-- Delete: payment (service d off the main account) --"
assert_tuple_absent features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 5e118fc6-b398-11f1-857a-6ae246604903 "service d removed from the main account's paid services"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 4f829a2c-b398-11f1-ba8e-6ae246604903 "main account still pays for service a (Delete only touches the filtered subject)"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 54c3d5a0-b398-11f1-9332-6ae246604903 "main account still pays for service b (Delete only touches the filtered subject)"

echo "-- Delete: desire (service c off team 2) --"
assert_tuple_absent rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 2 no longer desires service c"

echo "-- Unrelated state must be untouched by the Deletes --"
assert_tuple_exists rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 3 still desires service c (unrelated resource unchanged)"

# ---------------------------------------------------------------------------
# Final state: the deletes fully reversed the creates, so every tuple should
# match the first validation block from before the CreateTuples section. The
# assertions above each verified one slice of that; this tally covers every
# tuple assertion in the entire run.
# ---------------------------------------------------------------------------
print_assert_summary

# Exit non-zero when any assertion failed so scripted invocations can detect
# a failing run; exit zero when everything passed.
if [[ ${FAIL_COUNT} -gt 0 ]]; then
  exit 1
fi
