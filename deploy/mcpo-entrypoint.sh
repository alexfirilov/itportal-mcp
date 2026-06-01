#!/bin/sh
# Render an mcpo config pointing at an MCP server, then start mcpo.
#
# Two supported targets:
#   1. The itportal-mcp server directly (default) — auth via Authorization: Bearer.
#   2. A LiteLLM MCP gateway (preserves team scope) — set MCP_HEADERS_JSON to send
#      LiteLLM's auth header, e.g. {"x-litellm-api-key":"Bearer sk-itteamkey"}.
#
# Env:
#   MCP_SERVER_URL    target MCP URL (default http://itportal-mcp:8080/)
#   MCP_SERVER_TYPE   mcpo transport key (default streamable_http; some versions
#                     want "streamablehttp")
#   MCP_HEADERS_JSON  full headers object as JSON; overrides the default. Use this
#                     for the LiteLLM gateway. When unset, falls back to
#                     {"Authorization":"Bearer ${MCP_API_KEY}"} and MCP_API_KEY is
#                     then required.
#   MCPO_PORT         listen port (default 8000)
#   MCPO_API_KEY      optional key protecting mcpo's own REST surface
set -eu

: "${MCP_SERVER_URL:=http://itportal-mcp:8080/}"
: "${MCP_SERVER_TYPE:=streamable_http}"
: "${MCPO_PORT:=8000}"

if [ -n "${MCP_HEADERS_JSON:-}" ]; then
    HEADERS="$MCP_HEADERS_JSON"
else
    if [ -z "${MCP_API_KEY:-}" ]; then
        echo "mcpo-entrypoint: set MCP_HEADERS_JSON, or MCP_API_KEY for the default Authorization header" >&2
        exit 1
    fi
    HEADERS="{ \"Authorization\": \"Bearer ${MCP_API_KEY}\" }"
fi

CONFIG=/tmp/mcpo.config.json
cat > "$CONFIG" <<EOF
{
  "mcpServers": {
    "itportal": {
      "type": "${MCP_SERVER_TYPE}",
      "url": "${MCP_SERVER_URL}",
      "headers": ${HEADERS}
    }
  }
}
EOF

set -- mcpo --config "$CONFIG" --host 0.0.0.0 --port "$MCPO_PORT"
# Optionally protect mcpo's own REST surface with its own key.
if [ -n "${MCPO_API_KEY:-}" ]; then
    set -- "$@" --api-key "$MCPO_API_KEY"
fi

echo "mcpo-entrypoint: proxying ${MCP_SERVER_URL} (type=${MCP_SERVER_TYPE}) on :${MCPO_PORT}" >&2
exec "$@"
