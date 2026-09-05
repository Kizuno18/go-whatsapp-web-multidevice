feat: expose referral metadata in chat messages

## Context

Rebase of #641 by @juliomuhlbauer onto current main, with the test the review
asked for. His commit is cherry-picked, so authorship stays with him.

`referral_metadata` (Meta Ads / Click-to-WhatsApp attribution) has been stored
since migration 16 and both storage read paths — `GetMessages` and
`SearchMessages` — already select it, but `GET /chat/:jid/messages` never
returned it. This adds `ReferralMetadata` to `domainChat.MessageInfo`, maps it
in `GetChatMessages`, and documents it in the `ChatMessage` OpenAPI schema.

What changed against the original PR:

- Conflict resolution in the `MessageInfo` literal keeps everything main has
  gained since April — `SenderDisplayName` and the `Reactions` block — and only
  adds the one `ReferralMetadata: message.ReferralMetadata` line, so nothing in
  the response regresses.
- The openapi hunk moved with the schema: the original patched around line 4233,
  the `ChatMessage` schema now lives at ~5374. The property sits next to
  `call_metadata`, matching the Go field order.
- gofmt'd the struct — the original left `CallMetadata`/`ReferralMetadata`
  misaligned, which was CodeRabbit's nit.
- Parity check: `MessageInfo` is still built in exactly one place. The MCP
  `whatsapp_chat` / `messages` action returns the same
  `GetChatMessagesResponse` as structured output, so it picks the field up for
  free; the websocket path doesn't emit `MessageInfo`. No other wiring needed.

The field is typed `string` with `omitempty`, same as the sibling
`CallMetadata` — the raw JSON blob is passed through rather than parsed.

Supersedes #641.

## Test Results

`src/usecase/chat_test.go` gets `TestGetChatMessagesExposesReferralMetadata`: it
runs `GetChatMessages` over a stub repo holding one message with referral
metadata and one without, asserts the mapping, then marshals the response and
checks `referral_metadata` is present on the first and absent on the second, so
the `omitempty` tag is covered too. Dropping the mapping line makes it fail.

From `src/`:

- `gofmt -l .` — no new entries (three files are already unformatted on main and
  are left alone: `infrastructure/whatsapp/chatwoot_content_test.go`,
  `pkg/error/app_error.go`, `usecase/device_test.go`)
- `go build ./...` — pass
- `go vet ./...` — pass
- `go test ./...` — pass, 13 packages ok
