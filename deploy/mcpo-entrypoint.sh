#!/bin/sh
# Render an mcpo config that points at the itportal-mcp Streamable HTTP server,
# injecting the Bearer token from the environment, then start mcpo.
set -eu

: "${MCP_SERVER_URL:=http://itportal-mcp:8080/}"
# mcpo's config key for streamable HTTP has varied across versions
# (streamable_http / streamablehttp). Override via MCP_SERVER_TYPE if needed.
: "${MCP_SERVER_TYPE:=streamable_http}"
: "${MCPO_PORT:=8000}"

if [ -z "${MCP_API_KEY:-}" ]; then
    echo "mcpo-entrypoint: MCP_API_KEY is required (Bearer token for the MCP server)" >&2
    exit 1
fi

CONFIG=/tmp/mcpo.config.json
cat > "$CONFIG" <<EOF
{
  "mcpServers": {
    "itportal": {
      "type": "${MCP_SERVER_TYPE}",
      "url": "${MCP_SERVER_URL}",
      "headers": { "Authorization": "Bearer ${MCP_API_KEY}" }
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
