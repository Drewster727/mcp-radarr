# mcp-radarr

An MCP (Model Context Protocol) server that lets LLM agents interact with [Radarr](https://radarr.video/), the open-source movie library manager.

## Why it exists

Radarr has a capable API but no native way for an AI agent to use it. This server bridges that gap by exposing Radarr operations as MCP tools, allowing Claude (and any other MCP-capable agent) to look up, add, and manage movies through natural language.

## Tools

| Tool | Description | Always available |
|---|---|---|
| `lookup_movie` | Search TMDB via Radarr by title (+ optional year). Results include whether the movie is already in your library. | ✅ |
| `get_movies` | List library movies. Filterable by `downloaded`, `monitored`, and `year`. | ✅ |
| `add_movie` | Add a movie using the default quality profile. Returns a disambiguation list when multiple TMDB results match. | ✅ |
| `bulk_add` | Ensure multiple movies are in the Radarr library. Accepts a list of movies with title, optional year and tmdb_id. | ✅ |
| `update_movie` | Change a movie's monitored state or quality profile. | Requires `RADARR_ALLOW_MUTATIONS=true` |
| `delete_movie` | Remove a movie from Radarr (optionally deletes files). | Requires `RADARR_ALLOW_MUTATIONS=true` |

## Configuration

All settings are provided via environment variables.

| Variable | Required | Default | Description |
|---|---|---|---|
| `RADARR_API_KEY` | ✅ | — | Radarr API key (Settings → General) |
| `RADARR_ROOT_FOLDER_PATH` | ✅ | — | Root folder for new movies (e.g. `/movies`) |
| `RADARR_URL` | | `http://localhost:7878` | Base URL of your Radarr instance |
| `RADARR_DEFAULT_QUALITY_PROFILE` | | `Any` | Quality profile name assigned to new movies |
| `RADARR_ALLOW_MUTATIONS` | | `false` | Set to `true` to enable `update_movie` and `delete_movie` |

## Running locally

```bash
go build -o mcp-radarr ./cmd/server

RADARR_API_KEY=your_key \
RADARR_ROOT_FOLDER_PATH=/movies \
./mcp-radarr
```

## Docker

**Build:**

```bash
docker build -t mcp-radarr .
```

**Run:**

```bash
docker run --rm -i \
  -e RADARR_API_KEY=your_key \
  -e RADARR_ROOT_FOLDER_PATH=/movies \
  -e RADARR_URL=http://radarr:7878 \
  mcp-radarr
```

The server communicates over stdio (MCP's standard transport), so `-i` is required.

## Claude Desktop integration

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "radarr": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-e", "RADARR_API_KEY=your_key",
        "-e", "RADARR_ROOT_FOLDER_PATH=/movies",
        "-e", "RADARR_URL=http://host.docker.internal:7878",
        "mcp-radarr"
      ]
    }
  }
}
```

Or with the binary directly:

```json
{
  "mcpServers": {
    "radarr": {
      "command": "/path/to/mcp-radarr",
      "env": {
        "RADARR_API_KEY": "your_key",
        "RADARR_ROOT_FOLDER_PATH": "/movies",
        "RADARR_URL": "http://localhost:7878"
      }
    }
  }
}
```

## Claude Code integration

```bash
claude mcp add radarr /path/to/mcp-radarr \
  -e RADARR_API_KEY=your_key \
  -e RADARR_ROOT_FOLDER_PATH=/movies
```

## Tests

```bash
# Unit tests
go test ./internal/...

# Integration tests (mock Radarr server, no real Radarr needed)
go test -tags=integration ./test/integration/...
```
