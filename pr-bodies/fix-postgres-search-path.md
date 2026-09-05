docs: document postgres search_path via DB_URI

## Context

Closes #825.

Turns out this already works — no code change needed. `DB_URI` goes straight to
`sqlstore.New(ctx, "postgres", DBURI, dbLog)` in
`src/infrastructure/whatsapp/database.go`, nothing rewrites the query string, and
lib/pq forwards `search_path` as a startup parameter. So
`?search_path=whatsapp` puts every `whatsmeow_*` table in that schema today.

What is missing is documentation, plus two failure modes that are easy to hit and
whose error messages give no hint about the cause:

- the schema has to exist first, otherwise startup dies with
  `pq: no schema has been selected to create in (3F000)`
- the target database must not already have `whatsmeow_*` tables in some other
  schema. `dbutil`'s existence checks run
  `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)`
  with no `table_schema` filter, so a leftover set in `public` makes the version
  table check succeed against the wrong schema. With `search_path=whatsapp` that
  surfaces as `pq: relation "whatsmeow_version" does not exist (42P01)`; with
  `search_path=whatsapp,public` it silently keeps writing to `public`. Same
  reason `DB_KEYS_URI` cannot be a second schema in the same database — it needs
  its own database.

So this adds a `#### PostgreSQL schema isolation` subsection under the env var
table in `readme.md` and a three-line comment above `DB_URI` in
`src/.env.example`. No unit test — there is no new pure function to test, and the
behaviour under test is the driver's.

## Test Results

PostgreSQL 16 (Debian package, `service postgresql start`), whatsmeow
`v0.0.0-20260904121843-28bfe537ea6a`. A throwaway Go program called the repo's
own `whatsapp.InitWaDB` with each DSN, against a database reset between runs.

| DSN | Where the tables landed |
|---|---|
| `?search_path=whatsapp` (schema exists, DB otherwise empty) | `whatsapp` (17 tables) |
| `?search_path=whatsapp,public` (DB otherwise empty) | `whatsapp` (17 tables) |
| no `search_path` | `public` (17 tables) |
| `?search_path=whatsapp`, schema missing | startup fails, 3F000 |
| `?search_path=whatsapp`, `public` already populated | startup fails, 42P01 |
| `?search_path=whatsapp,public`, `public` already populated | silently stays on `public` |
| second container on `?search_path=wakeys` in the same DB | startup fails, 42P01 |

`information_schema` after the first run:

```
 table_schema | table_name
--------------+------------------------------------
 whatsapp     | whatsmeow_app_state_mutation_macs
 whatsapp     | whatsmeow_app_state_sync_keys
 whatsapp     | whatsmeow_app_state_version
 whatsapp     | whatsmeow_chat_settings
 whatsapp     | whatsmeow_contacts
 whatsapp     | whatsmeow_device
 whatsapp     | whatsmeow_event_buffer
 whatsapp     | whatsmeow_identity_keys
 whatsapp     | whatsmeow_lid_map
 whatsapp     | whatsmeow_message_secrets
 whatsapp     | whatsmeow_nct_salt
 whatsapp     | whatsmeow_pre_keys
 whatsapp     | whatsmeow_privacy_tokens
 whatsapp     | whatsmeow_retry_buffer
 whatsapp     | whatsmeow_sender_keys
 whatsapp     | whatsmeow_sessions
 whatsapp     | whatsmeow_version
(17 rows)
```

Not just DDL — a `PutDevice` / `GetAllDevices` round trip on the same container
returned the device and left `public` untouched:

```
ROUNDTRIP OK devices=1
 wa_devices | public_tables
------------+---------------
          1 |             0
```

The failure paths, verbatim:

```
Database initialization error: failed to upgrade database: failed to create
version table: pq: no schema has been selected to create in at column 14 (3F000)

Database initialization error: failed to upgrade database: pq: relation
"whatsmeow_version" does not exist at column 29 (42P01)
```

`gofmt -l` clean on the touched files, and `go build ./... && go vet ./... &&
go test ./...` pass from `src/`.
