# Upstream PR handoff

Branches are pushed to Kizuno18/go-whatsapp-web-multidevice. Open each compare
link, paste the matching body file (first line = PR title, rest = body).

## Round 1

| Branch | Target | Body file | Compare |
|---|---|---|---|
| fix/mcp-read-tools-text-fallback | closes #821 | pr-bodies/fix-mcp-read-tools-text-fallback.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:fix/mcp-read-tools-text-fallback?expand=1 |
| fix/extract-business-message-text | refs #823 | pr-bodies/fix-extract-business-message-text.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:fix/extract-business-message-text?expand=1 |
| feat/call-logs-endpoint | refs #796 | pr-bodies/feat-call-logs-endpoint.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:feat/call-logs-endpoint?expand=1 |
| feat/expose-referral-metadata | supersedes #641 | pr-bodies/feat-expose-referral-metadata.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:feat/expose-referral-metadata?expand=1 |

## Round 2

| Branch | Target | Body file | Compare |
|---|---|---|---|
| fix/postgres-search-path | closes #825 | pr-bodies/fix-postgres-search-path.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:fix/postgres-search-path?expand=1 |
| feat/per-device-webhook-ignore-groups | supersedes #756 | pr-bodies/feat-per-device-webhook-ignore-groups.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:feat/per-device-webhook-ignore-groups?expand=1 |
| fix/chatwoot-reopen-retry | builds on #822 | pr-bodies/fix-chatwoot-reopen-retry.md | https://github.com/aldinokemal/go-whatsapp-web-multidevice/compare/main...Kizuno18:go-whatsapp-web-multidevice:fix/chatwoot-reopen-retry?expand=1 |

Note on #822: the branch contains aguinaldotupy's commits plus ours on top. The PR
body offers him to cherry-pick our commit into his branch instead; if he does,
close ours.
