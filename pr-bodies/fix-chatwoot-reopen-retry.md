fix(chatwoot): persist failed conversation reopen for retry

Builds on #822 by @aguinaldotupy and adds the persisted reopen retry aldinokemal asked for; @aguinaldotupy feel free to pull this commit into your branch instead and I'll close this.

## Context

The reordering in #822 is right, but it leaves one corner open on the REST path. A history-sync message posts, Chatwoot has it, `UpsertChatwootMessageLink` stores the link — and only then does `toggle_status` run. If every reopen attempt in that pass fails and no later message in the same pass repairs it, the next sync filters those messages out as already linked, so nothing ever revisits the thread: it stays resolved with new messages sitting inside it. In-pass retries can't help, because the pass is the last thing that knows.

So the intent leaves the process. A failed reopen is enqueued on the existing `chatwoot_forward_queue` under a `chatwoot.conversation.reopen` event, and `StartChatwootForwardRetryWorker` replays it with the backoff it already has — no new table, no migration, no second worker. The queue's unique key is `(device_id, event_name, wa_message_id)`, so keying the row by `conversation:<id>` collapses repeated failures for one thread into a single row for free. The row carries a conversation id and account — never a message — so a replay physically cannot repost anything.

The part worth reviewing is when the retry declines to act, because a stale reopen is the same bug pointing the other way. Status alone can't tell "still resolved since we gave up" from "reopened by an inbound message and resolved again by an agent" — both read `resolved`, and toggling the second one undoes a deliberate decision. So the intent records when it was queued, and the worker reads `last_activity_at` alongside the status: activity newer than the intent means the resolve is the current decision and the row is dropped. That stamp also bounds the retry by wall clock (24h) rather than by attempts, which matters because `EnqueueChatwootForwardEvent`'s `ON CONFLICT` doesn't reset `attempts` — an attempt cap would let one exhausted row swallow every later failure on that conversation, whereas the conflict clause does rewrite `payload_json`, so a fresh failure restarts the window while the backoff carries on. The intent is dropped as well when the conversation is already open, when `CHATWOOT_REOPEN_CONVERSATION` has since been turned off, when the device no longer points at the account it was queued for (conversation ids are only unique per account), and on a permanent Chatwoot rejection. On success the row is deleted like any other completed job.

The new conversation read (`Client.GetConversationState`) tolerates the `{"payload": {...}}` wrapper the other decoders in `client.go` already handle, and treats a body with no status as an error rather than as "not resolved", so an odd response reschedules the row instead of silently deleting it.

The same fix covers the REST media pre-pass on the Postgres path, which posts and links the same way and whose rows pgimport then skips. `pgimport/writer.go` itself needs nothing: its reopen runs inside the import transaction, so a failure rolls the writes back and the next replay retries from scratch.

One small change to your tests, @aguinaldotupy: `restMediaPrePass` now takes the sync's `deviceID` (`syncChatPG` already had it) so the queued row is scoped the same way the rest of the sync is — the three call sites in `sync_reopen_test.go` just pass `msg.DeviceID`.

## Test Results

- `cd src && gofmt -l .` — only the three files already unformatted on main (`pkg/error/app_error.go`, `usecase/device_test.go`, `infrastructure/whatsapp/chatwoot_content_test.go`)
- `cd src && go build ./... && go vet ./...` — clean
- `cd src && go test ./...` — all packages pass; the four Postgres integration tests from #822 skip without a DB, as expected

New tests:

- `src/infrastructure/chatwoot/sync_reopen_retry_test.go` — message posts and links, every toggle fails, and one reopen intent lands on the queue with the right conversation/account/target status, a fresh enqueue stamp, and the real toggle error as `last_error`; two posted messages still produce one row; a successful in-pass reopen queues nothing; the media pre-pass queues under the sync's device; a re-enqueue refreshes the stamp instead of extending a dead one.
- `src/infrastructure/whatsapp/chatwoot_reopen_retry_test.go` — a due row replays as one `toggle_status` and zero message POSTs, then is cleared; the wrapped payload shape decodes; a body with no status reschedules rather than deletes; an already-open conversation is skipped; a resolve made *after* the intent is dropped while activity *predating* it still reopens; account mismatch, reopening disabled, an expired window and a row with no enqueue stamp each drop without touching Chatwoot.
- `src/infrastructure/chatstorage/sqlite_repository_chatwoot_test.go` — against the real SQLite queue: re-enqueueing the same conversation rewrites `payload_json` and `last_error` while `attempts` keeps counting, which is what makes the wall-clock window restart correctly.
