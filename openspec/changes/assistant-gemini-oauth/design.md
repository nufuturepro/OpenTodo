# Design

## Context

`internal/assistant` resolves one `Route` per turn: a provider name, a model and an unexported key, decided by `Store.Resolve` from the owner's settings, the member's own choice (`assistant_member_provider`) and the secrets file (`operator:<provider>`, `user:<id>:<provider>`). `Service.provider` turns that into an `AnthropicProvider` or a `ChatProvider`; `ChatProvider` posts to `https://generativelanguage.googleapis.com/v1beta/openai/chat/completions` with `Authorization: Bearer <key>`, streams SSE and normalises tool calls.

`internal/calsync` already runs Google's authorization-code flow with PKCE for calendar sync: an owner-configured client id (`calendar_sync.google_client_id` in the settings table) and secret (secrets file), a `calsync_oauth_state`-style pending row keyed by `StateHash`, a callback route with `signedIn.SessionOnly()`, and a token endpoint with refresh. `internal/calsync/google.go` is the template; this change reuses its shape in a leaf package rather than its code.

Google documents OAuth for the Gemini API ("Authentication with OAuth"): a bearer token from a user's Google account authenticates `generativelanguage.googleapis.com` with the `generative-language.retriever` scope (plus `cloud-platform` for the broader path), and `cloud-platform` is what Google's own samples request. Access tokens live one hour.

## Decisions

**D1. A credential, not a provider.** The provider stays `gemini`. `Route` gains an unexported credential that is either a static key or a token source, and an optional project. Resolution order in `Store.Resolve`, first match wins:

| Order | Condition | Credential | `Own` |
|---|---|---|---|
| 1 | member keys allowed, member chose a provider, own API key stored | the key | yes |
| 2 | member keys allowed, member chose a provider, own Google connection | the connection | yes |
| 3 | member keys allowed, a Google connection exists for the owner's default provider | the connection | yes |
| 4 | otherwise | the operator's key | no |

Adding a provider name would have meant a second copy of the disclosure, consent, cap and usage semantics for the same model; a credential leaves every one of those untouched. An own key beats a connection (order 1 before 2) so a person who pasted a key after connecting is not silently charged differently, and the Gemini card states the rule.

**D2. The owner's client, three fields.** Settings key `assistant.google_client_id` (plus `assistant.google_project`), secret `assistant.google_client_secret` in `<data>/secrets/assistant.json`. Separate from calendar sync's client: an instance may run one, the other, or both, and a person setting up the assistant must not be sent to configure calendars. Scopes: `openid`, `email` (to show which account is connected) and `https://www.googleapis.com/auth/generative-language.retriever`. If Google refuses generation for the retriever scope with `PERMISSION_DENIED`, `cloud-platform` joins the same consent in a follow-up — the scope constant is one place.

**D3. The connect flow.**

1. `POST /api/v1/me/assistant/google/start` mints `state` (32 bytes) and a PKCE verifier, stores `StateHash("assistant", state)` with the user id, verifier and redirect address for 10 minutes, and answers `{url}`. It refuses with 409 `google_not_configured` when the owner's client is incomplete.
2. The browser navigates to that URL. `GET /api/v1/assistant/google/callback` takes the state row, requires it to belong to the caller's session, exchanges the code, and stores the refresh token under `google:gemini:<user-id>`.
3. The account's address comes from the ID token claims (`email`); only the address is stored in `assistant_google_connection.account` for display.
4. The callback redirects to `/settings/integrations/assistant?assistant-google=connected|denied|invalid_state`, like `calsyncPanel`.

The refresh token is written only after the exchange succeeds; a failed callback leaves no row and no secret.

**D4. Tokens.** Access tokens are minted from the refresh token on demand and cached in memory for the assistant service, keyed by user id, dropped when they expire (one minute of slack). A turn mints before its request. On a 401 the adapter drops the cached token, mints once more and retries the request once; a second 401, or a refresh refused with `invalid_grant`, marks `last_error = "reconnect"` and answers `provider_auth` with a reconnect message instead of the API-key message. Nothing about the token is logged.

**D5. Project attribution.** With a user's OAuth token, Google bills the OAuth client's Cloud project unless the request names one. The connection carries an optional project (defaulting to the owner's `assistant.google_project`), sent as `x-goog-user-project`. Blank means no header, which is right when the client's project is the quota project. The form's help says the person's own project can be named when their account has one.

**D6. Data model.**

```sql
CREATE TABLE assistant_google_connection (
  user_id      TEXT PRIMARY KEY,
  account      TEXT NOT NULL DEFAULT '',
  project      TEXT NOT NULL DEFAULT '',
  connected_at INTEGER NOT NULL,
  last_error   TEXT NOT NULL DEFAULT ''
);
CREATE TABLE assistant_oauth_state (
  state_hash   TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL,
  verifier     TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  expires_at   INTEGER NOT NULL
);
```

Server state outside the op log, like consent, usage and member choices. Account deletion removes the row and the secret; no other entity changes.

**D7. Routes and policies.** One declared policy per route, registered through `routes.handle`, every one `.DemoDenied()` with matching entries in the demo deny list:

| Route | Policy |
|---|---|
| `GET /api/v1/settings/assistant/google` | `ownerOnly` |
| `PUT /api/v1/settings/assistant/google` | `ownerOnly(...).Credential()` |
| `POST /api/v1/me/assistant/google/start` | `signedIn().Recent()` |
| `GET /api/v1/assistant/google/callback` | `signedIn().SessionOnly()` |
| `GET /api/v1/me/assistant/google` | `signedIn` |
| `DELETE /api/v1/me/assistant/google` | `signedIn` |

Audit: `assistant_connection.created` and `assistant_connection.deleted` (the user id and provider `google`, never the address), and `settings.changed` with `assistant.google_client` unset/set for the owner's client.

**D8. Interface budget.** Nothing new in the chrome: the owner's three fields and the redirect address sit inside the existing Gemini card behind its own row, and Connect/Disconnect sits inside the existing "Which assistant you use" block. No new sidebar destination, no new screen, no new shortcut, no extra scroll region — the settings page keeps its one scroll area. Settings already loads lazily, so the first-load budget grows by the settings copy only.

**D9. The endpoint is verified, not assumed.** Google documents OAuth bearer tokens for the native `generateContent` methods and for Vertex's OpenAI-compatible endpoint, but not for `generativelanguage.googleapis.com/v1beta/openai`. The implementation keeps the OpenAI-compatible adapter (it already handles Gemini's schema subset and image refusals) and verifies a bearer token against it with a real client before release. If Google refuses it, the fallback is a native `:streamGenerateContent?alt=sse` adapter behind the same `Provider` interface, selected when the route's credential is a token; API keys keep the current adapter.

**D10. Consent and caps unchanged.** The provider string stays `gemini`, so an existing consent still counts and the disclosure list does not change. The disclosure's first paragraph gains the connected account as the destination wording ("with your Google account" instead of "with your key") — text only, not `disclosure_version`. Turns on a connection count as own turns for `cap_member_keys` and the own-key usage row, so the owner's cap behaves as it does for own keys.

**D11. Failure surfaces.** `google_not_configured` (owner has no client), `google_not_connected` (a turn or test needs a connection and there is none), `provider_auth` with the reconnect message (refresh refused), `invalid_state` (an expired or foreign sign-in), and `google_denied` (the person declined at Google). The first two are new codes for `web/src/lib/i18n/server-error-codes.json`.

## Verification (D9)

No Google Cloud project or Google account was available where this change was implemented, so the bearer-token question D9 leaves open is not answered yet. It is answered by one real turn, run like this:

1. Owner: Google Cloud project, Generative Language API enabled, OAuth client of type *Web application* with the redirect address the Assistant card shows, the owner under **Test users** while the client is in *Testing*; then the client id, secret and project in **Configure → Google accounts**.
2. A person: **My own provider and key → Google Gemini → Connect Google account**, then one message.
3. Record here whether the turn answers, and on a refusal the HTTP status, the error code the answer shows and Google's own error value from the server's log; no token is ever logged.

| What it proves | Expected |
| --- | --- |
| `v1beta/openai/chat/completions` accepts a user access token from the `generative-language.retriever` scope | the answer streams |
| `x-goog-user-project` aims the billing at the named project | no `403` about a missing or blocked quota project |
| the token is re-minted once after a `401` | no visible interruption in the first turn after the token's hour |

**Outcome: pending.** A refusal of the bearer token, or of the OpenAI-compatible path for it, means the native `generateContent`/`:streamGenerateContent?alt=sse` adapter in D9 is needed; an answer means the OpenAI-compatible adapter stays for both credentials and this section records the date and the model used.

## Size

About 8 KB of Go (the leaf package, the store, the adapter fields) and under 2 KB of gzip in the already-lazy Settings chunk. No dependency, no container image growth beyond the binary, no new runtime memory except one cached token per connected account.
