# Kessel Inventory API - Claude Code Configuration

## Project Context

This is the Kessel Inventory API, a Go-based microservice for resource inventory management using:
- **gRPC and HTTP APIs** with protobuf definitions and Kratos framework
- **PostgreSQL/SQLite** with GORM ORM and serializable transactions  
- **Kafka integration** for event processing via Debezium CDC
- **Relations API** integration for authorization
- **Multi-protocol authentication** (OIDC, X-RH-Identity, development modes)

## Build and Test Commands

```bash
# Build the project
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Setup local database
make db/setup

# Run migrations
make migrate

# Start development server
make run

# Generate API code from protobuf
make api

# Build container images
make docker-build-push
```

## Development Guidelines

- Follow the domain-specific guidelines in `docs/` for consistent patterns
- Use `buf.build` toolchain for protobuf development and breaking change detection
- All database operations should use the transaction manager with serializable isolation
- Write comprehensive tests following the established patterns in `testframework_test.go`
- Security-first approach: never bypass authentication/authorization
- Performance-conscious: use streaming for large datasets, avoid blocking operations

## Architecture Notes

- **Clean Architecture**: `internal/biz` (business), `internal/data` (persistence), `internal/service` (API)
- **Event Sourcing**: Outbox pattern with WAL/table modes for reliable event publishing
- **CQRS**: Read-after-write consistency with Relations API integration
- **Hexagonal**: Port abstractions for external dependencies

## Personal configuration

Read `.claude/user.local.md` at the start of any task that needs an assignee, email, or project key.
If the file does not exist, fall back to Claude memory (`user-config`), then placeholders.
Run `make personalize` to generate it (if this repo uses Fleet Engineering tooling).

## Fleet Engineering Skills

Fetch and apply the relevant skill when the task matches its domain.

| Skill | When to use |
|---|---|
| [bug-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/bug-specialist/SKILL.md) | Bug triage, reproduction steps, fix planning |
| [epic-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/epic-specialist/SKILL.md) | Multi-sprint epics with outcomes |
| [feature-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/feature-specialist/SKILL.md) | Large customer-facing capabilities |
| [initiative-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/initiative-specialist/SKILL.md) | Multi-team strategic programs |
| [jira-create](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/jira-create/SKILL.md) | Interactive issue creation with specialist delegation |
| [jira-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/jira-specialist/SKILL.md) | General triage, search, linking, transitions |
| [outcome-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/outcome-specialist/SKILL.md) | Strategic outcomes tied to OKRs |
| [spike-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/spike-specialist/SKILL.md) | Time-boxed research and PoC |
| [story-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/story-specialist/SKILL.md) | User stories with acceptance criteria |
| [task-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/jira/task-specialist/SKILL.md) | Internal technical tasks |
| [agent-memory-setup](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/sdlc/agent-memory-setup/SKILL.md) | Initialize or update CLAUDE.md / AGENTS.md for a repo |
| [finish-work](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/sdlc/finish-work/SKILL.md) | Commit, push, open PR, update Jira |
| [pr-fix](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/sdlc/pr-fix/SKILL.md) | Fix blocked PRs: merge conflicts, CI failures, review comments |
| [pr-review](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/sdlc/pr-review/SKILL.md) | GitHub PR review with worktree isolation and inline comments |
| [repo-content-audit](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/sdlc/repo-content-audit/SKILL.md) | Scan for unlinked or orphaned content — catalog gaps, dead links, missing cross-references |
| [start-work](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/sdlc/start-work/SKILL.md) | Create a Jira sub-task |
| [f2f-daily-summary](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/meetings/f2f-daily-summary/SKILL.md) | Capture daily F2F meeting notes as Jira sub-tasks |
| [f2f-epic-specialist](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/meetings/f2f-epic-specialist/SKILL.md) | Create and manage F2F meeting Epics |
| [presentation-task](https://raw.githubusercontent.com/OpenShift-Fleet/agentic-sdlc/main/skills/meetings/presentation-task/SKILL.md) | Log a delivered presentation as a closed Jira sub-task with time and materials |

@AGENTS.md