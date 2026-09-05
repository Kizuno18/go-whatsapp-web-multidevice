feat(webhook): ignore group messages per instance, not just globally

Picks up #756 by @huboperacional: rebased onto main and adds the null-reset the review
asked for. His eight commits are cherry-picked as-is, so authorship stays with him; my
three follow-ups sit on top.

## Context

The feature itself is unchanged from #756: a nullable per-device `webhook_ignore_groups`
override, stored on the `devices` table, surfaced on `PATCH`/`GET
/devices/{device_id}/webhook`, and consulted by the webhook forwarder only for `@g.us`
JIDs. `NULL` means "never configured" and the device keeps following the global
`WHATSAPP_WEBHOOK_IGNORE_JIDS` list.

What changed relative to the original PR:

- **Migration renumbered 44 -> 45.** Main grew its own migration 44 (poll definitions)
  while #756 was open, so the `ALTER TABLE devices ADD COLUMN webhook_ignore_groups` is
  now appended as 45. The only other rebase conflict was an append clash in
  `sqlite_repository_test.go` / `device_test.go`; both sides were kept.

- **`null` now clears the override.** This was the maintainer's outstanding item:
  binding the field into a `*bool` collapses "key absent" and "key present but `null`",
  so once a device had `true` or `false` there was no request that could put the column
  back to `NULL`. The handler decodes into `utils.Nullable[bool]` (`Set`/`Valid`/`Value`,
  a small `json.Unmarshaler` in `src/pkg/utils/nullable.go`) and the three states now
  mean what you would expect:

  ```json
  {"webhook_url": "https://hook.example.com"}                                 // keeps the stored override
  {"webhook_url": "https://hook.example.com", "webhook_ignore_groups": null}  // clears it -> global list applies
  {"webhook_url": "https://hook.example.com", "webhook_ignore_groups": false} // this device keeps its groups
  ```

  The repository already wrote a nil `*bool` as SQL `NULL`, so nothing changed below the
  handler.

- **Codex's multi-slot lookup ambiguity is real, and fixed.** The webhook payload's
  `device_id` is the bare NonAD JID, and `GetDeviceRecordByJID` deliberately returns nil
  once two slots share that number (#760) — so on exactly the multi-instance setup this
  feature is for, sibling slots resolved no override at all. The forwarder now resolves
  the record by the AD JID of the slot in the event context (`DeviceFromContext`, set per
  event in `event_handler.go`), which addresses one row, and falls back to the bare JID
  for slots with no AD JID recorded yet (logging the lookup error, not swallowing it).
  Same pass also resolves the device record once instead of twice, so the webhook config
  and the group override can no longer come from two different rows; that left
  `getWebhookConfigForDevice` without a production caller, so it is gone and its tests
  now exercise the live path.

- **An opt-out no longer un-ignores exact group JIDs.** In #756 `webhook_ignore_groups:
  false` skipped the ignore-list match for group JIDs altogether, so a group the operator
  had listed by its exact JID in `WHATSAPP_WEBHOOK_IGNORE_JIDS` came back. The override
  now replaces only the `@g.us` wildcard, which is the contract the review asked for and
  the one the docs describe (`utils.MatchesExactIgnoredJID`).

- **Docs.** `webhook_ignore_groups` was missing from `docs/openapi.yaml` entirely; it is
  now on the PATCH body and on both response bodies, with the tri-state spelled out.
  `docs/webhook-payload.md` gets a bullet in the ignore-JID behaviour list. `readme.md`
  documents only the global env/flag, so it is untouched.

Supersedes #756.

## Test Results

From `src/`:

- `gofmt -l .` — only the three files already unformatted on main
  (`pkg/error/app_error.go`, `usecase/device_test.go`,
  `infrastructure/whatsapp/chatwoot_content_test.go`).
- `go build ./...` — ok
- `go vet ./...` — ok
- `go test ./...` — all packages pass, including huboperacional's original REST, storage
  and forwarder tests unchanged.

New tests on top of his:

- `TestUpdateDeviceWebhook_IgnoreGroupsTriState` — omitted key keeps the stored value,
  `null` clears it, explicit `false` sets it (the `true -> null -> nil` cycle the review
  asked for).
- `TestSQLiteRepositoryDeviceWebhookConfig_IgnoreGroupsClearsToNull` — a set override
  written back as nil round-trips as `NULL` through both `GetDeviceWebhookConfig` and
  `GetDeviceRecordByJID`.
- `TestWebhookIgnoreJID_DeviceOverrideResolvedByADJID` — with the bare number ambiguous,
  the override is still found via the emitting slot's AD JID.
- `TestWebhookIgnoreJID_DeviceOverrideFalseScopeIsTheWildcard` — with `false`, an exact
  group JID in the global list is still ignored while the `@g.us` wildcard is neutralized.
- `TestNullableUnmarshalJSON` / `TestNullableMarshalJSON` / `TestNullableRejectsMismatchedType`,
  `TestMatchesExactIgnoredJID`.
