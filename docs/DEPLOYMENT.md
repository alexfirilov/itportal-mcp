# Deployment — Open WebUI via LiteLLM (production: benarit)

How the ITPortal MCP is wired into the existing Open WebUI + LiteLLM stack so the
**IT team** in Open WebUI can use ITPortal tools, with all tool traffic routed
**through LiteLLM** (team scope + spend logging preserved).

## Topology

```
Open WebUI backend ──> mcpo ──> LiteLLM  /itportal/mcp ──> itportal-mcp ──> ITPortal API
 (Global tool server)  (OpenAPI    (MCP gateway,           (this repo)      (v2.1)
                        ->MCP shim)  IT-team key)
```

Why each hop:
- **Open WebUI v0.9.x tool servers are OpenAPI-only** — they cannot consume an MCP
  server (or LiteLLM's MCP gateway) directly. `mcpo` is the OpenAPI⇄MCP translator.
- **mcpo points at the LiteLLM gateway, not at itportal-mcp directly** — so LiteLLM
  still enforces the IT-team boundary and logs spend on tool calls.
- **Registered as a Global (admin) tool server**, not a per-user one. Per the Open
  WebUI docs, User tool servers are called from the **browser** (can't resolve the
  Docker name / mixed-content over HTTPS); **Global** tool servers are called from
  the **backend**, which reaches `itportal-mcp`/`mcpo` by container name.

## Host: 10.80.99.20 (`admin-alexf`)

- Stack repo (Open WebUI + LiteLLM): `/opt/benarit-ai-chat` (compose project
  `benarit-ai-chat`, network `benarit-ai-chat_aichat-net`).
- This repo: `/opt/itportal-mcp` (cloned from GitHub).
- Containers (all `restart: unless-stopped`, on `aichat-net`):
  - `itportal-mcp` — host port **8090**→8080, `/healthz` probe, snapshot of the live portal.
  - `itportal-mcpo` — host port **8000**, OpenAPI surface for Open WebUI.
  - `litellm` (4000), `open-webui` (8080), plus postgres/redis.
- TLS/ingress: nginx-proxy-manager fronts `*.benarit.com` (ai., litellm., …). The MCP
  and mcpo containers are **not** publicly exposed — only reached backend-side.

## /opt/itportal-mcp configuration

`.env` (chmod 600) — key entries:
```
ITPORTAL_BASE_URL=https://itportal-benarit.nofcloud.co.il
ITPORTAL_API_KEY=<portal token>
ITPORTAL_API_VERSION=2.1
MCP_API_KEY=<random; itportal-mcp transport secret>
MCP_HOST_PORT=8090
SNAPSHOT_DEVICE_LIMIT=5000
# mcpo, routed through the LiteLLM gateway:
MCPO_HOST_PORT=8000
MCP_SERVER_URL=http://litellm:4000/itportal/mcp
MCP_SERVER_TYPE=streamable_http
MCP_HEADERS_JSON={"x-litellm-api-key":"Bearer <IT-team virtual key>"}
MCPO_API_KEY=<random; guards mcpo's REST surface>
```

`docker-compose.override.yml` joins the existing stack network:
```yaml
networks:
  default:
    name: benarit-ai-chat_aichat-net
    external: true
```

Bring up: `cd /opt/itportal-mcp && docker compose up -d itportal-mcp mcpo`

## LiteLLM (`/opt/benarit-ai-chat`)

`litellm/config.yaml` — appended:
```yaml
mcp_servers:
  itportal:
    url: "http://itportal-mcp:8080/"
    transport: "http"
    auth_type: "api_key"
    auth_value: os.environ/ITPORTAL_MCP_KEY
```
`.env`: `ITPORTAL_MCP_KEY=<itportal-mcp MCP_API_KEY>`.
`docker-compose.override.yml`: surfaces `ITPORTAL_MCP_KEY` into the litellm container.

**Team scope:** the **IT** team (`team_alias: IT`) has
`object_permission.mcp_servers = ["itportal"]`, so its virtual keys inherit the
tools. A durable IT-team key (alias `itportal-mcpo`) is what mcpo sends as
`x-litellm-api-key`.

Backups taken before edits: `litellm/config.yaml.bak.*`, `.env.bak.*`.

## Open WebUI global tool server

Stored in Open WebUI's config DB at `tool_server.connections` (equivalently the
`TOOL_SERVER_CONNECTIONS` PersistentConfig / Admin → Settings → Tools):
```json
{
  "type": "openapi",
  "url": "http://itportal-mcpo:8000/itportal",
  "path": "openapi.json",
  "auth_type": "bearer",
  "key": "<MCPO_API_KEY>",
  "config": {"enable": true},
  "info": {"id": "itportal", "name": "ITPortal"}
}
```
A restart of `open-webui` loads it (`Initialized 1 tool server(s)`). `webui.db`
backed up before editing.

> Caveat: a Global tool server is visible to **all** Open WebUI users (every call
> still flows through the IT-team LiteLLM key). To restrict *which OWUI users* see
> it, set the connection's access-control to an IT user-group.

## Verify

```bash
# itportal-mcp healthy + by-name reachable
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8090/healthz          # 200
docker exec open-webui curl -s -o /dev/null -w '%{http_code}\n' http://itportal-mcp:8080/healthz   # 200

# tools through the full chain (mcpo -> litellm -> mcp); MCPO key from .env
K=$(grep '^MCPO_API_KEY=' /opt/itportal-mcp/.env | cut -d= -f2-)
curl -s -X POST -H "Authorization: Bearer $K" -H 'Content-Type: application/json' \
  -d '{"query":"Benarit"}' http://localhost:8000/itportal/itportal-search_docs | head -c 120

# Open WebUI loaded the global tool server
docker logs --since 5m open-webui 2>&1 | grep -i "tool server"
```

In a chat: pick a model, enable the **ITPortal** tool, ask e.g. "search ITPortal
for Benarit devices."

## Auth header shapes (why it connects)

`itportal-mcp` accepts the shared secret as `Authorization: Bearer <key>`, a raw
`Authorization: <key>`, or `X-API-Key: <key>` (see `cmd/server/main.go`). This lets
LiteLLM's `auth_type: api_key` authenticate regardless of how it forwards the key.
LiteLLM's MCP gateway in turn accepts `x-litellm-api-key: Bearer <virtual key>`.
