# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o mcp-radarr ./cmd/server

# Unit tests
go test ./internal/...

# Integration tests (spins up a mock Radarr server — no real Radarr required)
go test -tags=integration ./test/integration/...

# Single test
go test ./internal/radarr/ -run TestGetMovies -v

# Sync dependencies after changing go.mod
go mod tidy
```

## Architecture

The server is an MCP stdio server. The entry point (`cmd/server/main.go`) loads config, creates a `radarr.Client`, and hands both to `mcpserver.New`, which registers tools and calls `server.ServeStdio`.

```
cmd/server/main.go          entry point — wires config → client → MCP server
internal/config/            env-var loading; required: RADARR_API_KEY, RADARR_ROOT_FOLDER_PATH
internal/radarr/
  models.go                 Go structs for Radarr API v3 (Movie, QualityProfile, …)
  client.go                 Thread-safe HTTP client with pooled transport; all Radarr API calls live here
  client_test.go            Unit tests using httptest.NewServer mocks
internal/mcpserver/
  server.go                 MCP server init + tool registration; mutation tools gated on cfg.AllowMutations
  handlers.go               One handler per tool; add_movie parallelises lookup + profile fetch via goroutines
  handlers_test.go          Unit tests for all handler logic
test/integration/
  integration_test.go       End-to-end tests against a stateful in-memory mock Radarr; build tag: integration
```

## Key design decisions

- **Read-only by default.** `update_movie` and `delete_movie` are only registered when `RADARR_ALLOW_MUTATIONS=true`. No runtime guard is needed inside the handlers — unregistered tools are invisible to the agent.
- **Parallel fan-out in `add_movie`.** The TMDB lookup and quality profile resolution run concurrently via `sync.WaitGroup` + buffered channels, since neither depends on the other.
- **Connection pooling.** `radarr.NewClient` configures `http.Transport` with `MaxIdleConns=100`, `MaxConnsPerHost=10`, and HTTP/2. The client is safe for concurrent use.
- **Disambiguation on ambiguous titles.** When `/movie/lookup` returns multiple results and no year is provided, the handler returns a candidate list rather than picking blindly.
- **MCP library:** `github.com/mark3labs/mcp-go` v0.20.0. `CallToolResult.IsError` is a plain `bool` (not `*bool`). Tool content is `[]mcp.Content`; assert to `mcp.TextContent` to read text.
