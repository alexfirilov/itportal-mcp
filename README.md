# ITPortal MCP Server

[Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that exposes your ITPortal documentation to AI agents (Claude, etc.) over HTTP.

---

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `ITPORTAL_BASE_URL` | Yes | — | Base URL of your ITPortal instance, e.g. `https://itportal.example.com` |
| `ITPORTAL_API_KEY` | Yes | — | ITPortal API token (Admin Settings → Generate API Key) |
| `MCP_API_KEY` | Yes | — | Secret Bearer token clients must send to access this server |
| `MCP_LISTEN_ADDR` | No | `:8080` | TCP address the HTTP server binds to |
| `SNAPSHOT_REFRESH_INTERVAL` | No | `30m` | How often the documentation snapshot is rebuilt (Go duration, e.g. `15m`, `1h`) |
| `SNAPSHOT_LIMIT_PER_ENTITY` | No | `1000` | Max records fetched per entity type when building the snapshot |

Create a `.env` file in the project root — it is loaded automatically at startup, or just copy `.env.example` to `.env` and fill in real values.

```env
ITPORTAL_BASE_URL=https://itportal.example.com
ITPORTAL_API_KEY=your-itportal-token
MCP_API_KEY=a-strong-random-secret
```
---

## Running

```bash
# Build
go build -o itportal-mcp ./cmd/server

# Run (reads .env automatically)
./itportal-mcp
```

Or without a binary:

```bash
go run ./cmd/server
```

The server blocks until the initial documentation snapshot is built, then starts accepting connections.

---

## Connecting a client

The server exposes a single Streamable HTTP endpoint at `/`. Configure your MCP client:

```json
{
  "mcpServers": {
    "itportal": {
      "type": "http",
      "url": "http://localhost:8080/",
      "headers": {
        "Authorization": "Bearer <MCP_API_KEY>"
      }
    }
  }
}
```

---

## Security

### Transport authentication
Every HTTP request must carry `Authorization: Bearer <MCP_API_KEY>`. Requests without a valid token are rejected with `401`/`403` before reaching the MCP layer.

### Sensitive fields intentionally excluded from the snapshot

The documentation snapshot served to AI clients is scrubbed of credentials. The following fields are **never** included in the snapshot markdown or any MCP resource:

| Entity | Excluded fields |
|---|---|
| Device credentials | `password`, `2faCode` |
| Accounts | `password`, `2faCode` |
| Additional credentials | `password` |

These fields remain accessible only via `get_entity_details` called explicitly by an authorised agent — they are never bulk-exported into the context cache.

### Network exposure
By default the server listens on all interfaces (`:8080`). For production, either:
- Bind to `127.0.0.1:8080` via `MCP_LISTEN_ADDR` and front with a reverse proxy (nginx, Caddy) that terminates TLS, or
- Run inside a private network with no public exposure.

**Do not expose the server to the public internet without TLS.**
