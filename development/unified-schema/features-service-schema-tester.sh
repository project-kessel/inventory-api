#!/bin/bash
#
# End-to-end tester for the features service model (JSON and unified schema) against a
# running inventory-api instance.
#
# Prerequisites:
#   - Full Kessel stack running in unified schema mode (via
#     `make kessel-up-unified` or an ephemeral deployment); the unified-schema
#     config enables load-reporter-representation, which reporter-side
#     relationship tuples require
#   - `zed` on PATH and configured (`zed context set`) against the same
#     SpiceDB instance inventory-api writes to
#   - `grpcurl` on PATH for the API calls
#
# What this script does, in order:
#   1. Seeds a workspace hierarchy directly into SpiceDB via zed
#   2. Reports billing accounts and workspaces via ReportResource
#   3. Verifies the enabled_services permission with Check calls
#   4. Asserts the expected SpiceDB tuples with targeted PASS/FAIL checks
#   5. Exercises AddResourceRelationships, then re-verifies Check + tuples
#   6. Exercises RemoveResourceRelationships, then re-verifies Check + tuples
#      (the removes reverse the adds, returning SpiceDB to its pre-Add state)
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
  echo "./features-service-schema-tester.sh PORT_NUMBER"
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
# To eyeball the raw tuples manually while debugging a FAIL, run:
#   zed relationship read features/billing_account
#   zed relationship read rbac/workspace
# ---------------------------------------------------------------------------

PASS_COUNT=0
FAIL_COUNT=0

# assert_tuple_exists RESOURCE RELATION SUBJECT_ID DESCRIPTION
#   Checks that the given relation on the resource has a tuple whose subject
#   matches SUBJECT_ID.
#   PASS = the expected tuple exists in SpiceDB.
#   FAIL = the expected tuple is missing; the tuples that ARE present are
#   printed to help pinpoint what went wrong.
assert_tuple_exists() {
  local actual
  actual=$(zed relationship read "$1" "$2" 2>/dev/null)
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
  if ! actual=$(zed relationship read "$1" "$2" 2>/dev/null); then
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
#   asserts that the response's "allowed" value equals EXPECTED_ALLOWED
#   ("true" or "false"). PASS/FAIL is recorded like the tuple assertions; a
#   failed grpcurl call is recorded as a failure rather than a pass.
assert_check() {
  local expected="$1" description="$2"
  local response
  if ! response=$(grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.Check 2>&1); then
    echo "FAIL: $description — Check call failed"
    echo "      error:"
    echo "$response" | sed 's/^/        /'
    FAIL_COUNT=$((FAIL_COUNT + 1))
    return
  fi
  if echo "$response" | grep -Eq "\"allowed\": *${expected}"; then
    echo "PASS: $description — allowed=${expected}, as expected"
    PASS_COUNT=$((PASS_COUNT + 1))
  else
    echo "FAIL: $description — expected allowed=${expected}"
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
# team 4         c3d4e5f6-b396-11f1-9c1d-6ae246604903  (never reported; auto-created by AddResourceRelationships)

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
echo "==================================================================="
echo "Testing ReportResource for Billing Accounts and Features Workspaces"
echo "==================================================================="

# Each ReportResource below upserts a full snapshot of one resource. T
# The reported state sets up both sides of the access model:
#   payment:  main account pays for services a+b, special account pays for c
#   desire:   root wants a, dept a wants b, dept b links the special account,
#             team 1 wants d, team 3 wants c (team 2 wants nothing)
echo "Main account pays for: service a, service b"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "billing_account",
    "reporterType": "features",
    "reporterInstanceId": "1b9aa216-b39a-11f1-8e5d-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "3055cc32-b398-11f1-bdd0-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "services": ["4f829a2c-b398-11f1-ba8e-6ae246604903", "54c3d5a0-b398-11f1-9332-6ae246604903"]
      }
    }
}
EOF

echo "special account pays for: service c"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "billing_account",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "3ba27996-b398-11f1-bf17-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "services": ["59be7c18-b398-11f1-9781-6ae246604903"]
      }
    }
}
EOF

echo "root workspace uses main account, subscribes service a"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "31a6d1a6-b395-11f1-adbd-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_billing_account": "3055cc32-b398-11f1-bdd0-6ae246604903",
        "direct_service_preferences": ["4f829a2c-b398-11f1-ba8e-6ae246604903"]
      }
    }
}
EOF

echo "dept a subscribes service b"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "8adc4a2a-b396-11f1-8aa5-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_service_preferences": ["54c3d5a0-b398-11f1-9332-6ae246604903"]
      }
    }
}
EOF

echo "dept b uses special account"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "9015d2ae-b396-11f1-9308-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_billing_account": "3ba27996-b398-11f1-bf17-6ae246604903"
      }
    }
}
EOF

echo "team 1 subscribes service d"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "9554d7a6-b396-11f1-8fa2-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_service_preferences": ["5e118fc6-b398-11f1-857a-6ae246604903"]
      }
    }
}
EOF

echo "team 3 subscribes service c"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "representations": {
      "metadata": {
        "localResourceId": "a0af6abc-b396-11f1-ae2b-6ae246604903",
        "apiHref": "https://apiHref.com/",
        "consoleHref": "https://www.console.com/",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_service_preferences": ["59be7c18-b398-11f1-9781-6ae246604903"]
      }
    }
}
EOF

echo "All resources reported...time for Checks"

# Replication is asynchronous: ReportResource committed representations and
# outbox events; the consumer still needs to diff versions and write the
# tuples into SpiceDB. This sleep waits for that pipeline to settle before
# any Check or zed read happens.
echo "Need to give things time to replicate...sleeping for 20 seconds"
sleep 20

# Each Check below evaluates the enabled_services PERMISSION and is asserted
# by assert_check, which reads the request from the heredoc, invokes the
# endpoint, and verifies the response's "allowed" value against the expected
# outcome — recording PASS/FAIL like the tuple assertions.
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
  }
}
EOF

# ---------------------------------------------------------------------------
# Tuple assertions: verify the state that ReportResource + the consumer
# produced, one PASS/FAIL line per expected tuple. Covers both sides of the
# access model — payment (billing account #t_services) and desire
# (workspace #t_features_workspace_*) — plus the zed-seeded hierarchy.
# ---------------------------------------------------------------------------

echo "Validating reported relationships landed in SpiceDB..."

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
echo "========================================================"
echo "Testing the AddResourceRelationships endpoint"
echo "========================================================"

# The three adds below each demonstrate a different behavior: the first two
# fix the missing side of a check that was DENIED in the first round (one adds
# the missing payment, the other the missing desire), and the third targets a
# workspace that has NEVER been reported — AddResourceRelationships must
# auto-create the resource from the provided relationship fields alone.
echo "Adding service D to the main billing account's paid services"
echo "(Team 1 already desires D directly; it was denied above because nobody paid for it)"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService/AddResourceRelationships <<EOF
{
    "type": "billing_account",
    "reporterType": "features",
    "reporterInstanceId": "1b9aa216-b39a-11f1-8e5d-6ae246604903",
    "localResourceId": "3055cc32-b398-11f1-bdd0-6ae246604903",
    "reporter": {
      "services": ["5e118fc6-b398-11f1-857a-6ae246604903"]
    }
}
EOF

echo "Adding service C as a direct preference on Team 2's workspace"
echo "(Dept B's special account already pays for C; Team 2 was denied above because it did not desire it)"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService/AddResourceRelationships <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "localResourceId": "9a4cd31c-b396-11f1-af2e-6ae246604903",
    "reporter": {
      "direct_service_preferences": ["59be7c18-b398-11f1-9781-6ae246604903"]
    }
}
EOF

echo "Adding service C as a direct preference on Team 4's workspace (AUTO-CREATE test)"
echo "(Team 4 has never been reported via ReportResource; the Add must create it)"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService/AddResourceRelationships <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "localResourceId": "c3d4e5f6-b396-11f1-9c1d-6ae246604903",
    "reporter": {
      "direct_service_preferences": ["59be7c18-b398-11f1-9781-6ae246604903"]
    }
}
EOF

# Team 4's hierarchy link is seeded directly with zed (like step 1): the
# auto-created resource gets its desire tuple from the Add above, but the
# t_parent link to dept b is needed so inherited PAYMENT (dept b's special
# account paying for service c) can reach team 4 during permission checks.
zed relationship create rbac/workspace:c3d4e5f6-b396-11f1-9c1d-6ae246604903 t_parent rbac/workspace:9015d2ae-b396-11f1-9308-6ae246604903  # team 4 to dept b

# Same async replication wait as after ReportResource
echo "Giving the deltas time to replicate...sleeping for 20 seconds"
sleep 20

# Both previously-DENIED checks should now flip to allowed, because each Add
# satisfied the missing half of the desire+payment equation.

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
  }
}
EOF

# Auto-created resource: team 4 was created by the Add alone, and once its
# t_parent link landed it satisfies both sides of the model for service c —
# desire (its own preference) + payment (inherited from dept b's special
# account). If auto-create is broken, this check fails AND the tuple grep
# below reports a missing preference.
assert_check true "Team 4 (auto-created) SHOULD have access to service C (own desire + dept b's account pays for it)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "c3d4e5f6-b396-11f1-9c1d-6ae246604903",
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
  }
}
EOF

echo "Validating the Add relationships landed in SpiceDB..."

echo "-- Add: payment merge (service d onto the main account) --"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 5e118fc6-b398-11f1-857a-6ae246604903 "main account now pays for service d"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 4f829a2c-b398-11f1-ba8e-6ae246604903 "main account STILL pays for service a (Add merges, does not replace)"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 54c3d5a0-b398-11f1-9332-6ae246604903 "main account STILL pays for service b (Add merges, does not replace)"

echo "-- Add: desire (service c onto team 2) --"
assert_tuple_exists rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 2 now desires service c"

echo "-- Add: auto-create (team 4, never reported) --"
# The auto-created resource's relationship must replicate exactly like a
# reported one: the consumer diffs the initial (empty) representation against
# the version the Add created and writes the tuple.
assert_tuple_exists rbac/workspace:c3d4e5f6-b396-11f1-9c1d-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "auto-created team 4 desires service c"
assert_tuple_exists rbac/workspace:c3d4e5f6-b396-11f1-9c1d-6ae246604903 t_parent 9015d2ae-b396-11f1-9308-6ae246604903 "team 4's hierarchy link to dept b (zed-seeded)"

echo "-- Unrelated state must be untouched by the Adds --"
assert_tuple_exists rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 3 still desires service c (unrelated resource unchanged)"

echo ""
echo "========================================================"
echo "Testing the RemoveResourceRelationships endpoint"
echo "========================================================"

# The three removes below reverse the adds from the previous section,
# returning SpiceDB to its pre-Add state. The last one also exercises Remove
# against the auto-created team 4 resource.
echo "Removing service D from the main billing account's paid services"
echo "(This reverses the Add above; Team 1 desires D but nobody pays for it again)"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService/RemoveResourceRelationships <<EOF
{
    "type": "billing_account",
    "reporterType": "features",
    "reporterInstanceId": "1b9aa216-b39a-11f1-8e5d-6ae246604903",
    "localResourceId": "3055cc32-b398-11f1-bdd0-6ae246604903",
    "reporter": {
      "services": ["5e118fc6-b398-11f1-857a-6ae246604903"]
    }
}
EOF

echo "Removing service C from Team 2's direct preferences"
echo "(This reverses the Add above; Dept B's account still pays for C but Team 2 no longer desires it)"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService/RemoveResourceRelationships <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "localResourceId": "9a4cd31c-b396-11f1-af2e-6ae246604903",
    "reporter": {
      "direct_service_preferences": ["59be7c18-b398-11f1-9781-6ae246604903"]
    }
}
EOF

echo "Removing service C from Team 4's direct preferences"
echo "(This reverses the auto-create Add; Team 4 keeps existing but desires nothing again)"
grpcurl -plaintext -d @ localhost:${PORT_NUMBER} kessel.inventory.v1beta2.KesselInventoryService/RemoveResourceRelationships <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "3ba27996-b398-11f1-bf17-6ae246604903",
    "localResourceId": "c3d4e5f6-b396-11f1-9c1d-6ae246604903",
    "reporter": {
      "direct_service_preferences": ["59be7c18-b398-11f1-9781-6ae246604903"]
    }
}
EOF

# Same async replication wait
echo "Giving the deltas time to replicate...sleeping for 20 seconds"
sleep 20

# The three checks that flipped to allowed after the Adds should now flip
# back to denied (team 4 included — the auto-created resource), and the
# final check confirms an unrelated relationship was untouched.
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
  }
}
EOF

assert_check false "Team 4 should NO LONGER have access to service C (auto-created resource, preference removed)" <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "c3d4e5f6-b396-11f1-9c1d-6ae246604903",
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
  }
}
EOF

echo "Validating the removed relationships are gone from SpiceDB..."

echo "-- Remove: payment (service d off the main account) --"
assert_tuple_absent features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 5e118fc6-b398-11f1-857a-6ae246604903 "service d removed from the main account's paid services"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 4f829a2c-b398-11f1-ba8e-6ae246604903 "main account still pays for service a (Remove only touches named values)"
assert_tuple_exists features/billing_account:3055cc32-b398-11f1-bdd0-6ae246604903 t_services 54c3d5a0-b398-11f1-9332-6ae246604903 "main account still pays for service b (Remove only touches named values)"

echo "-- Remove: desire (service c off team 2) --"
assert_tuple_absent rbac/workspace:9a4cd31c-b396-11f1-af2e-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 2 no longer desires service c"

echo "-- Remove: auto-created resource (service c off team 4) --"
assert_tuple_absent rbac/workspace:c3d4e5f6-b396-11f1-9c1d-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "auto-created team 4 no longer desires service c"
assert_tuple_exists rbac/workspace:c3d4e5f6-b396-11f1-9c1d-6ae246604903 t_parent 9015d2ae-b396-11f1-9308-6ae246604903 "team 4's hierarchy link to dept b remains (zed-seeded, not managed by the API)"

echo "-- Unrelated state must be untouched by the Removes --"
assert_tuple_exists rbac/workspace:a0af6abc-b396-11f1-ae2b-6ae246604903 t_features_workspace_direct_service_preferences 59be7c18-b398-11f1-9781-6ae246604903 "team 3 still desires service c (unrelated resource unchanged)"

# ---------------------------------------------------------------------------
# Final state: the removes fully reversed the adds, so every tuple should
# match the first validation block from before the Add section, with ONE
# expected leftover — team 4's zed-seeded t_parent link, which the API has no
# endpoint to remove. The assertions above each verified one slice of that;
# this tally covers every tuple assertion in the entire run.
# ---------------------------------------------------------------------------
print_assert_summary

# Exit non-zero when any assertion failed so scripted invocations can detect
# a failing run; exit zero when everything passed.
if [[ ${FAIL_COUNT} -gt 0 ]]; then
  exit 1
fi
