fix(message): extract text from business template and interactive messages

## Context

Chats from WhatsApp Business / Cloud API senders render blank (#823) because
`ExtractMessageTextFromProto` only knows about `conversation`, `extendedTextMessage`,
media captions and the *response* variants of buttons/lists/templates. Business
senders put their body somewhere else entirely, so extraction returned `""`.

That empty string is not cosmetic: both `sqlite_repository.StoreMessage` and
`history_sync` skip a message when content and media type are both empty, so the
message is never stored at all — hence a chat that exists but shows nothing, while
WhatsApp Web renders it fine. Webhook payloads lost `body` the same way.

Message types now handled, preferring body text, then title, then footer:

- `TemplateMessage` — hydrated template and the `hydratedFourRowTemplate` oneof,
  plus the legacy `fourRowTemplate` (content/title/footer HSM) and
  `interactiveMessageTemplate`
- `HighlyStructuredMessage` — via `hydratedHsm`; a non-hydrated HSM only carries an
  element name and substitution params, so it stays empty on purpose
- `InteractiveMessage` — body text, header title/subtitle, footer
- `InteractiveResponseMessage` — body text
- `ButtonsMessage` — content text, header text, footer text
- `ListMessage` — description, title, footer text
- `ProductMessage` — body, product title/description, catalog title, footer
- `OrderMessage` — message, order title

`BuildEventMessage` gets the same fallback so webhook `body` and stored history stay
in parity. Unknown types still return `""`.

One caveat worth flagging: the issue has no raw payload or log, so this is based on
the proto types the Cloud API actually sends rather than a captured repro.
@akashgurnani, could you confirm your business chats fill in after this? If some are
still blank, the exact `Message` field that arrives would pin it down. Marked
`Refs #823` rather than `Closes` for that reason.

## Test Results

From `src/`:

- `gofmt -l .` — only the three files that were already unformatted on `main`
  (`infrastructure/whatsapp/chatwoot_content_test.go`, `pkg/error/app_error.go`,
  `usecase/device_test.go`); nothing new.
- `go build ./...` — clean
- `go vet ./...` — clean
- `go test ./...` — all packages pass

New table-driven cases in `pkg/utils/whatsapp_test.go` cover hydrated templates
(content text and title fallback), the legacy four-row template, hydrated and
non-hydrated HSM, interactive body and header fallback, interactive response,
buttons content and header text, list description and title, product, order, and an
unknown type that must stay empty. Plus one `BuildEventMessage` case for the webhook
path.

No live business-account test — I don't have a WABA sender to send from.
