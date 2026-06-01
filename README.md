# ITPortal MCP Server

[Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that exposes your ITPortal documentation to AI agents (Claude, etc.) over HTTP. Targets the **ITPortal REST API v2.1**.

---

## Environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `ITPORTAL_BASE_URL` | Yes | — | Base URL of your ITPortal instance, e.g. `https://itportal.example.com` |
| `ITPORTAL_API_KEY` | Yes | — | ITPortal API token (Admin Settings → Generate API Key). Sent as HTTP Basic auth (key as password). |
| `ITPORTAL_API_VERSION` | No | `2.1` | ITPortal REST API version. Set `2.0` only for legacy instances. |
| `ITPORTAL_ENCRYPTION_KEY` | No | — | Custom credential-encryption key. Required only if your org uses custom encryption, to read/write credential endpoints. |
| `MCP_API_KEY` | Yes | — | Secret Bearer token clients must send to access this server |
| `MCP_LISTEN_ADDR` | No | `:8080` | TCP address the HTTP server binds to |
| `SNAPSHOT_REFRESH_INTERVAL` | No | `30m` | How often the documentation snapshot is rebuilt (Go duration, e.g. `15m`, `1h`) |
| `SNAPSHOT_LIMIT_PER_ENTITY` | No | `1000` | Max records fetched per entity type when building the snapshot |
| `SNAPSHOT_DEVICE_LIMIT` | No | = `SNAPSHOT_LIMIT_PER_ENTITY` | Separate cap for devices (usually the largest entity set) |

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

### Open WebUI (via mcpo)

Open WebUI consumes **OpenAPI tool servers**, not MCP directly. The `mcpo` compose
service wraps this MCP server and re-exposes its tools as REST:

```
Open WebUI → OpenAPI → mcpo → HTTP+Bearer → itportal-mcp
```

```bash
docker compose up -d            # starts both itportal-mcp and mcpo
```

`mcpo` listens on `MCPO_HOST_PORT` (default `8000`), Swagger UI at
`http://<host>:8000/docs`. In Open WebUI, add it under **Settings → Tools** as an
OpenAPI server pointing at `http://<host>:8000` (send `Authorization: Bearer
<MCPO_API_KEY>` if you set one). `mcpo` authenticates to the MCP server itself using
`MCP_API_KEY` from `.env`.

> If `mcpo` can't connect, your version may expect `MCP_SERVER_TYPE=streamablehttp`
> (no underscore). Note: only **tools** are exposed over REST — MCP resources such as
> `itportal://snapshot` are not, so use `search_docs`/`list_entities` from Open WebUI.

To route Open WebUI tool calls **through a LiteLLM MCP gateway** (so a LiteLLM team
scopes the tools), point `mcpo` at the gateway instead of the MCP server:
`MCP_SERVER_URL=http://litellm:4000/<server>/mcp` and
`MCP_HEADERS_JSON={"x-litellm-api-key":"Bearer <team-key>"}`. Register the mcpo URL
in Open WebUI as a **Global** tool server (Admin → Settings → Tools) — global servers
are called backend-side, so they can reach container names; per-user servers are
browser-side and cannot. The full production wiring is in
[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).

---

## Tools & resources

The server exposes one cached resource and a set of tools.

**Resource** — `itportal://snapshot`: the full documented environment as Markdown.
Read it once per conversation to load everything into context (prompt-cached). JSON
sub-resources are also available: `itportal://companies`, `itportal://sites`,
`itportal://devices`, `itportal://kbs`, `itportal://contacts`.

**Read tools**
- `search_docs` — keyword search across the cached snapshot.
- `list_entities` — live, filtered, cursor-paginated lists. Types: company, site, device,
  kb, contact, account, agreement, document, facility, cabinet, configuration, ipnetwork,
  address, form, additional_credential, user, country, security_group, main_contact,
  kb_category, device_type, template.
- `get_entity_details` — one record plus sub-resources (device IPs/notes/management URLs).
- `get_credentials` — stored secrets for an account/device/configuration (on demand).
- `get_logs` — audit logs (userAccess, adminAccess, loginLogout, passwordAccess, passwordChanges).

**Write tools**
- `create_device`, `create_kb_article`, `create_entity` (generic), `add_device_ip`,
  `add_device_note`, `add_interaction`, `upload_file`.
- `update_entity`, `delete_entity`.
- `manage_relationship` — link two objects (symmetric invLinks).
- `manage_folder`, `manage_folder_file` — per-object document trees + file upload/download.
- `manage_credential` — additional credentials attached to any object.
- `manage_type` — custom type lists (per kind).
- `manage_kb_category` — KB categories and subcategories.
- `refresh_snapshot` — force a snapshot rebuild.

> `docs/api_spec.json` is the legacy v2.0 reference. `docs/test-portal-api.ps1` is the
> authoritative exercise of the live v2.1 surface. `docs/IMPROVEMENT_PLAN.md` records the
> v2.1 upgrade design.

---

## Development

```bash
go build ./...     # compile
go vet ./...       # static checks
go test ./...      # unit tests (httptest-mocked API; no live ITPortal needed)
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

Secrets are never bulk-exported into the context cache. They are returned only when an
authorised agent explicitly calls `get_credentials` (account/device/configuration) or
`manage_credential` (additional credentials) — and, for custom-encryption orgs, only when
`ITPORTAL_ENCRYPTION_KEY` is configured. Treat those two tools as privileged.

### Network exposure
By default the server listens on all interfaces (`:8080`). For production, either:
- Bind to `127.0.0.1:8080` via `MCP_LISTEN_ADDR` and front with a reverse proxy (nginx, Caddy) that terminates TLS, or
- Run inside a private network with no public exposure.

**Do not expose the server to the public internet without TLS.**
