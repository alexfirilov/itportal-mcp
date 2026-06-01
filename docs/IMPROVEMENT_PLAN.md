# ITPortal MCP — v2.1 Upgrade & Cleanup Plan

Source of truth for v2.1 behaviour: `docs/test-portal-api.ps1`. Legacy reference: `docs/api_spec.json` (v2.0).

## Phase 1 — Client core (protocol migration)
- Configurable API version (`ITPORTAL_API_VERSION`, default `2.1`). `do()` rewrites the
  `/api/2.0/` prefix in internal path literals to the configured version, so existing
  call-sites stay untouched.
- Auth: build `Authorization: Basic base64(":"+apiKey)` unless the key already carries a
  scheme (`Basic `/`Bearer `).
- Optional `X-Encryption-Key` header (`ITPORTAL_ENCRYPTION_KEY`) for credential endpoints.
- PATCH uses `application/merge-patch+json`; POST/others use `application/json`.
- Cursor pagination: `listPage` returns `pageMeta{Total,Count,NextCursor}`; `listAll`
  loops on `nextCursor`. `ListOptions.Cursor` added.
- Creates: POST → parse `Location` header for the new id; top-level entities then GET the
  full record (for URL); sub-resources return `{id}` + echoed input.

## Phase 2 — Models
- Fix: IPNetwork `networkAddress`; Interaction `note`.
- Add: Device `hostName`; DeviceMUrl `preferred`; Company `contact`; Address `company`;
  AdditionalCredential `type` + `portalObject{itemType,id}`.
- New: Relationship(+Target), Folder, FolderFile, KBSubCategory, PortalObjectRef, LogEntry.

## Phase 3 — New client methods
Relationships, Folders, FolderFiles (multipart), Type management (create/update/delete),
KB category + subcategory management, company-scoped device/document lists,
configuration credentials, top-level addresses CRUD, logs, additionalCredentials create,
interactions list/create, generic Delete for every entity.

## Phase 4 — MCP tools
- New: `manage_relationship`, `manage_folder`, `list_folder_files`, `manage_type`,
  `manage_kb_category`, `add_interaction`, `manage_credential`, `delete_entity`, `get_logs`.
- Extend: `list_entities` (+ relationships/folders/logs/types/interactions/addresses),
  `create_entity` (+ address), `upload_file` (+ folder file target).
- `get_entity_details` enriched (relationships, interactions where relevant).

## Phase 5 — Config / wiring / docs
`config.go` (+version, +encryption key), `main.go` (client ctor), `server.go`
(instructions + tool registration), README, `.env.example`, `Dockerfile` parity.

## Phase 6 — Tests
`httptest`-based unit tests for: auth header, version rewrite, cursor pagination,
create-from-Location, merge-patch content type, encryption header, markdown render.
`go build ./... && go vet ./... && go test ./...`.
