feat(call): add call logs listing endpoint

## Context

Closes the read side of #796: incoming calls are already persisted, but there is
no way to get at them without paging every chat.

`handleCallOffer` stores each `call.offer` through `CreateIncomingCallRecord` as
a synthetic message row with `media_type = "call"` and a small `call_metadata`
JSON blob. Today the only way to read those back is `GET /chat/:jid/messages`
per chat, filtering `media_type == "call"` client-side — which means you need to
know the chat first, and you page through unrelated messages to find calls.

`GET /call/logs` lists those records for the device in context, newest first,
with `limit` (default 25, max 100), `offset` and an optional `chat_jid` filter.
`call_metadata` is parsed into fields instead of being handed back as a JSON
string, so `call_id` can be fed straight into `POST /call/reject`.

Layering follows the existing chat list path: handler parses the query,
`ValidateListCallLogs` applies the limit/offset bounds, and the usecase resolves
the device from context before calling storage. Storage side is a new
`GetCallRecords(*CallRecordFilter)` on `IChatStorageRepository`, the SQLite
repository and `chatstorage_wrapper.go`, filtering `device_id = ?` plus
`media_type = 'call'` and returning the unpaginated total for the pagination
echo. No migration — it queries rows that already exist.

```
$ curl -s -H 'X-Device-Id: 628123456789@s.whatsapp.net' \
    'http://localhost:3000/call/logs?limit=2'
{
  "code": "SUCCESS",
  "message": "Success get call logs",
  "results": {
    "data": [
      {
        "id": "call:ABC123DEF456",
        "chat_jid": "6289685028129@s.whatsapp.net",
        "sender_jid": "6289685028129@s.whatsapp.net",
        "timestamp": "2026-08-22T10:05:00Z",
        "is_from_me": false,
        "call_metadata": {
          "call_id": "ABC123DEF456",
          "auto_rejected": true,
          "remote_platform": "android"
        }
      }
    ],
    "pagination": { "limit": 2, "offset": 0, "total": 12 }
  }
}
```

Deliberately not covered, since it needs event handling this PR does not touch:

- outgoing calls — only `events.CallOffer` is wired up, so `is_from_me` is
  always false today
- duration and outcome (answered/ended/missed) — `CallAccept` and
  `CallTerminate` aren't handled, so there is nothing to report
- backfill of calls from before the device was linked; history sync does not
  carry call records
- no MCP tool, and no `search` param (the stored content is a fixed
  "Incoming call" string, so matching on it buys nothing)

This is a first slice: it exposes what is already stored. Enriching
`call_metadata` from the other call events is the natural follow-up, and the
endpoint shape doesn't change when it lands.

## Test Results

From `src/`:

- `gofmt -l .` — no new entries (`pkg/error/app_error.go`,
  `usecase/device_test.go` and `infrastructure/whatsapp/chatwoot_content_test.go`
  are already unformatted on main)
- `go build ./...` — ok
- `go vet ./...` — clean
- `go test ./...` — all packages pass

New tests:

- `src/infrastructure/chatstorage/sqlite_repository_call_test.go` — against a
  real in-memory-ish SQLite repo: only `media_type = 'call'` rows come back,
  only for the requested device, ordered newest first; limit/offset paging and
  the `chat_jid` filter; missing `device_id` is rejected
- `src/validations/call_validation_test.go` — table test for the limit default
  (25), the 1..100 bounds and the offset floor
