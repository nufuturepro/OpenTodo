# Tasks

## 1. Server

- [x] 1.1 `internal/googleoauth`: client, PKCE, namespaced state hashes, authorization URL, token endpoint with exchange/refresh and the in-memory access-token cache; verify `go test ./internal/googleoauth/` (PKCE verifier/challenge pair, state hash collisions across namespaces, exchange and refresh shapes, `invalid_grant` classification, cache expiry)
- [x] 1.2 Migration `0056_assistant_google_oauth` adding `assistant_google_connection` and `assistant_oauth_state`; verify `go test ./internal/store/...` (the migration tests pass; `TestLockIsExclusive` fails on Windows only, a pre-existing file-lock limitation)
- [x] 1.3 `internal/assistant/google.go`: the owner's client (id, secret, project) and the person's connection (save, read, delete, reconnect state), the refresh token in the secrets file, access-token minting; verify `go test ./internal/assistant/ -run Google` (connect, refresh, revoked grant, deletion removes the secret)
- [x] 1.4 `Route` carries a credential and a project; `Store.Resolve` follows the order of design D1; an own key still wins over a connection; verify `go test ./internal/assistant/ -run Resolve`
- [x] 1.5 `ChatProvider` takes a token source and a project, sends `x-goog-user-project`, drops and re-mints once after a 401; verify `go test ./internal/assistant/ -run 'ChatProvider|GoogleToken'` against a fake `generativelanguage` server (bearer header, project header, refresh-once, second 401 is `provider_auth` with the reconnect message)

## 2. Routes

- [x] 2.1 Six routes in `internal/httpapi/assistant_google.go` with their declared policies, demo-denied, plus the demo deny list entries; verify `go test ./internal/httpapi/ -run 'AssistantGoogle|Policy|Demo'` (401 anonymous, 403 member on the owner's client, session-only callback, foreign state 400, denied consent, connected status, disconnect)
- [x] 2.2 `GET /api/v1/settings/assistant` reports the client's presence and project, never the secret; `GET /api/v1/me/assistant` reports `google_available`, `google_connected`, `google_account`, `google_project`; verify `go test ./internal/httpapi/ -run AssistantGoogle`
- [x] 2.3 Audit: `assistant_connection.created`/`.deleted` in the closed catalogue and `assistant.google_client` as `settings.changed`; verify `go test ./internal/audit/...` and that no address or token reaches the log

## 3. Client

- [x] 3.1 `web/src/lib/api.ts`: the six calls and the new status fields; `server-error-codes.json` gains `google_not_configured` and `google_not_connected`
- [x] 3.2 Owner Gemini card: client id, client secret, project and the copyable redirect address; verify `assistantSettings.test.tsx` and a story for configured and not configured (`AssistantGoogleClientSetUp`, `AssistantGoogleClientNotSetUp`)
- [x] 3.3 "Which assistant you use": Connect Google account, connected address with Disconnect, the note that an own API key wins, and no connect option for encrypted accounts; verify `assistantSettings.test.tsx` (connect, disconnect, connected, reconnect state). `make ui-lint` cannot run green on Windows: its allow list matches `web/src/...` and the reported paths use backslashes, so every exception misses (pre-existing)
- [x] 3.4 Consent disclosure names the Google account when a connection is used; verify `assistantConsent.test.tsx`
- [x] 3.5 Translations in `messages.json` and the five catalogs; verify `make i18n-check`. On Windows the script skips `shortcuts.ts` (its path test uses `/`), which reports every shortcut string as unused; with that file included the five catalogs check clean at 7105 messages

## 4. Documentation and release

- [x] 4.1 `docs/assistant-setup.md`, `docs/assistant-owner-options.md` and `docs/assistant-settings.md` link the new reference page `docs/assistant-google.md` (the guide pages were at the 600-word cap, so the client setup, tokens and costs live there); `docs/assistant-privacy.md` and `docs/ai-limits.md` for the connection and the counting; verify `make docs-check`
- [x] 4.2 `docs/api.md` lists the six routes and their errors
- [ ] 4.3 Verify a real bearer token against `generativelanguage.googleapis.com/v1beta/openai/chat/completions` (design D9) and record the outcome in this change; fall back to a native streaming adapter if it is refused — needs a Google Cloud project and a person's consent, so it cannot be done from this checkout
- [x] 4.4 CHANGELOG entry under Unreleased (Added: Gemini with a Google account)
- [ ] 4.5 `go test ./...`, `cd web && npm test && npm run build`, `make check` with TZ=UTC — `go build ./...`, `go vet`, the new and affected Go tests, `tsc`, `npm run build`, `docs-check`, `bundle-budget` and `openspec validate` pass here; `go test ./...` still fails on Windows-only assertions in test files this change does not touch (`README`-mode 0600 checks, file locks, path parsing)
