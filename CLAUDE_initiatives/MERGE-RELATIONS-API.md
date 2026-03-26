# Merge Relations-API into Inventory-API

This is the inventory-api codebase for the Kessel project. We are currently executing **Phase 2** of a multi-phase plan to merge relations-api into inventory-api.

## Related Codebases

- **Inventory API**: https://github.com/project-kessel/inventory-api (this repository)
- **Relations API**: https://github.com/project-kessel/relations-api (source for lift-and-shift work)

## Overall Plan

The complete initiative plan is documented in this Google Doc:
https://docs.google.com/document/d/1_VZvitlp7Db2AQbXqoe5KnqR3o35ekXsAgJDhi1ALk4/edit

**Parent Epic**: [RHCLOUD-44628](https://issues.redhat.com/browse/RHCLOUD-44628) - Merge Kessel Inventory and Relations

## Current Phase: Phase 2 – Expand inventory-api

**Lead time**: 2 sprints (not high effort but depends on customer availability, agreement on endpoints, code reviews, etc.)

### Objectives

1. **New endpoints**: Propose, discuss, and implement new endpoints with corresponding call-outs to relations-api
   - Propose/discuss new endpoints with the team (e.g. proto spec + expected semantics and behaviour)
   - Implement endpoints with call-outs to relations-api (assumes no/minimal changes to relations-api)
   - Update customer SDKs
   - Actively track and assist customers with migration to the new SDKs/API

2. **Embedded relations repository**: Lift, shift and embed the spicedb repository from relations-api
   - Lift, shift and embed the spicedb repository + tests from relations-api as a relations repository in inventory-api
   - Lift and shift relations config settings/options, preshared keys, secrets, schema configmap, etc.
   - Wire up a switch/feature toggle, so inventory-api can be configured to either call out to relations-api, or use the new embedded repository

### Customer Requirements (from Phase 1)

Analysis of relations-api consumers:
https://docs.google.com/spreadsheets/d/1SrhiWuJYvsYwzYRrUq52htpEE1ft2zAq-8WdDbBV8Ak/edit?usp=sharing

## Jira Issue Configuration

When creating Jira issues for this initiative, use the following invariants:

- **Project**: RHCLOUD
- **Parent Issue**: RHCLOUD-44628
- **Component**: Fabric Kessel
- **Work Type**: Product / Portfolio Work (ID: 10610)
- **Summary Prefix**: "Phase <phase number>: "

### Implementation Notes

- Work Type "Product / Portfolio Work" maps to ID `10610` in the Jira API
- Link type for related issues is "Related" (not "Relates")
- Component was previously "Management Fabric" - now updated to "Fabric Kessel"
- Legacy label "mgmt-fabric-kessel" has been removed

## Standard Operating Procedures (SOPs)

### Keeping Context Fresh

**Last initiative plan read**: 2026-03-25

When reading this file:
1. Check the "Last initiative plan read" date above
2. If the date is > 1 day old (or not set), re-read the initiative plan at: https://docs.google.com/document/d/1_VZvitlp7Db2AQbXqoe5KnqR3o35ekXsAgJDhi1ALk4/edit
3. Review for any changes to objectives, phases, or requirements
4. Prompt the user to update this file and related context if changes are found
5. Update the "Last initiative plan read" date to today's date

### Updating Jira Status

When instructed to update Jira issue status or transitions:

1. Perform the requested Jira update via the jira-mcp-server tools
2. Update the "Currently Active Work" section of this file:
   - Add issues that are moved to active statuses ("In Progress", "Code Review", etc.)
   - Remove issues that are moved to completed or blocked status
   - Include the current status in parentheses for clarity (e.g., "(Code Review)")
   - Keep this section as a simple list of what's currently being worked on

### Proto Creation and Editing

When creating or modifying protocol buffer definitions:

1. **Follow official protobuf best practices**: https://protobuf.dev/best-practices/dos-donts/
2. **Scan existing protos** in `api/kessel/inventory/v1beta2/` to understand and align with established patterns
3. **Reuse existing types** before creating new ones - maintain consistency across the API

### Verifying Code Changes

Before considering work complete, all code changes must be verified:

1. **Build**: Ensure code compiles
   ```bash
   make local-build
   # CI uses: make build (includes FIPS settings, won't work locally)
   ```

2. **Lint**: Ensure code passes formatting and linting
   ```bash
   goimports -w <changed-files>
   # CI uses: make lint (requires Docker, may fail locally)
   ```

3. **Tests**: Run tests
   ```bash
   go test ./...
   # Alternative: make test (may have coverage reporting issues locally but tests run)

   # For targeted testing:
   go test -run TestSpecificCase ./path/to/package
   ```

4. **Proto changes**: If proto files were modified, regenerate code
   ```bash
   make api
   ```

**Note**: The CI pipeline runs full `golangci-lint` and `make build` with FIPS. Local verification helps catch issues early but doesn't guarantee CI will pass. Address any CI failures promptly.

## Next Phases (Future Work)

- **Phase 3**: Functional testing and performance
- **Phase 4**: Operationalize (dashboards, alerts, SOPs, documentation)
- **Phase 5**: Offboard relations-api (switch to embedded repository, scale down, clean up)

## Currently Active Work

- **RHCLOUD-46115** - Phase 2: Evaluate LookupResources endpoint requirements for inventory-api (Code Review)
- **RHCLOUD-39498** - Phase 2: Add Lookup Subjects to v1beta2 (In Progress)

**Note**: For all issues related to this initiative, query Jira using parent epic [RHCLOUD-44628](https://issues.redhat.com/browse/RHCLOUD-44628).

### Customer Endpoint Analysis

From Phase 1 investigation, the following endpoints need to be implemented:
- **LookupSubjects** - Required by Notifications and RBAC teams
- **LookupResources** - Required by Notifications and RBAC teams
- **Check** - Required by Notifications and RBAC teams

Status: Endpoint proposals/discussion completed and marked as "Good" in the plan.
