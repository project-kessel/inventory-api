# Cursor IDE: Postgres MCP

This repository includes a [Model Context Protocol](https://modelcontextprotocol.io/) configuration so Cursor can connect to the local development PostgreSQL instance used by the inventory stack.

## Prerequisites

1. **Database**: Start the dev stack so `invdatabase` is listening on **localhost:5433**. See `development/docker-compose.yaml` and [docker-compose-options.md](./docker-compose-options.md).
2. **Node.js**: The MCP server is loaded on demand via `npx` (`mcp-postgres`). Install a current Node.js (OS packages or [nvm](https://github.com/nvm-sh/nvm)).

## Configuration

Workspace MCP is defined in `.cursor/mcp.json` at the repository root. The `DATABASE_URL` matches the default `invdatabase` settings in `development/docker-compose.yaml` (`POSTGRES_DB=spicedb`, port **5433**, same `POSTGRES_PASSWORD` as in that file).

## Troubleshooting

### `env: 'node': No such file or directory`

Cursor may launch MCP servers with a minimal `PATH`, so the `npx` launcher cannot find `node`. Prepend your Node.js `bin` directory to the `PATH` value in `.cursor/mcp.json` (for example the directory containing `node` when you run `which node` with your usual shell environment).

Alternatively, invoke Node explicitly by setting `command` to the full path to `node` and putting the full path to `npx` as the first argument in `args`, followed by `-y` and `mcp-postgres`.

### Connection refused or MCP offline

Ensure Compose has started Postgres and **5433** is published to the host. The MCP only works while that database is reachable.
