fix(mcp): include JSON payload in read tool text results

## Context

Text-only MCP clients (Claude among them) never read `structuredContent`, and our
read tools put the payload nowhere else — the `content` text block was just a
summary like "Retrieved 3 chats (offset 0, limit 25)". So those tools are
effectively blind on such clients. Closes #821.

Added `structuredWithJSON` in `src/ui/mcp/result.go`, which keeps the existing
human summary and appends the marshalled payload to the same text block (the
structured content is unchanged, so clients that do read it see exactly what they
saw before). If marshalling fails it falls back to the summary alone.

Swapped at the read call sites only:

- `src/ui/mcp/chat.go` — `list_chats`, `list_contacts`, `get_messages`
- `src/ui/mcp/group.go` — `info`, `participants`, `join_requests`
- `src/ui/mcp/app.go` — `status`

Write actions (send, create, join, leave, archive, mark read, participant
changes, ...) still return a summary; their text already describes the outcome
and dumping the response JSON there adds nothing. `invite_link` and
`download_media` are left alone too — the link and the file path are already in
their summary text.

## Test Results

- `gofmt -l .` — clean for `ui/mcp` (the three files it lists,
  `pkg/error/app_error.go`, `usecase/device_test.go`,
  `infrastructure/whatsapp/chatwoot_content_test.go`, are unformatted on main
  already and untouched here)
- `go build ./...`, `go vet ./...`, `go test ./...` all pass from `src/`
- New `src/ui/mcp/result_test.go`: table test over `structuredWithJSON` (map,
  slice, nil and an unmarshalable payload) plus handler tests asserting
  `list_chats` and group `participants` return text containing the serialized
  payload, and that `archive` still returns the summary only
