# Proposal

## Why

The assistant already speaks to Google Gemini, but only through an API key: the owner's in the secrets file, or a member's own when the owner allows it (`assistant-member-provider`). Getting a Gemini key means opening AI Studio, creating a project, minting a key and pasting it into a settings form — and that key is a long-lived bearer token with no expiry, no account identity and no per-person revocation. People who already use Google for everything expect to sign in with their Google account instead, and the server already has the machinery: calendar sync has run a Google authorization-code + PKCE flow with per-user connections since `calendar-two-way`.

The free "log in with the Gemini CLI" route is deliberately not this change. Google's Gemini CLI terms name third-party software reaching the Code Assist backend as a violation that "may be grounds for suspension", and accounts have been suspended for it. OpenTodo talks to the public Gemini API with an OAuth client the owner created.

## What Changes

- **A Google account is a second credential for the Gemini provider.** Settings → Integrations → Assistant gains, in the Gemini card, a **Connect Google account** action. The person is sent to Google, comes back connected, and their turns then go out with a short-lived OAuth access token instead of an API key. **Disconnect** removes the stored refresh token at the server and revokes nothing else.
- **The owner configures one OAuth client.** Three owner fields — client id, client secret and an optional Google Cloud project — and the redirect address to register at Google, shown next to them. Calendar sync's Google client is a separate setting; setting up one does not imply the other.
- **Per person, like the calendar connections.** Every account connects its own Google account, so an administrator never has to share a credential and a person can take their connection away. The existing `allow_member_keys` switch governs this too, now read as "people may bring their own key or connect their own Google account".
- **The connection is a credential, not a provider.** The provider stays `gemini` and the model stays the owner's or the member's; consent, the disclosure, the monthly cap and the usage rows do not change shape. An own API key still wins over a connection, and the screen says so.
- **Refreshed and revoked.** Access tokens live an hour and are kept in memory only. A refused refresh marks the connection; the person sees *Reconnect* and turns say so instead of "check your API key".
- **Nothing else goes to Google.** Requests still go only to `generativelanguage.googleapis.com`, plus Google's token endpoint for the exchange and the refresh. No new environment variable, no new mandatory setting, no dependency.

Not **BREAKING**: no op field, no sync format, no route removal.

## Out of scope

- Vertex AI (`aiplatform.googleapis.com`) and its project/location routing. The Gemini API path is what the current adapter, base URL and model ids already use.
- OAuth for Anthropic or OpenAI, and OAuth clients that are not Google's.
- The Gemini CLI / Code Assist free tier: Google's terms forbid third-party use and it would put every user's account at risk.
- Sharing one Google connection across an instance (an org-wide credential), and OAuth for the on-device assistant of encrypted accounts (the device calls providers itself, with its own key).
- Adopting `internal/googleoauth` inside `internal/calsync`. Its Google and Microsoft flows keep their own code in this change; switching them is a separate cleanup with its own tests.

## Roadmap

Advances the in-app assistant's provider setup in ROADMAP.md's "Assistant" row and the AI-agents pillar of `docs/ai-and-agents.md`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `in-app-assistant`: a Google account as an alternative credential for the Gemini provider, the owner's OAuth client, the connect/disconnect routes, token refresh and the reconnect state.

## Impact

- **Data model, sync, op schema, E2EE:** none. Two new server tables outside the op log: `assistant_google_connection` (who connected, the account's address, an optional project, when) and `assistant_oauth_state` (a pending sign-in). The refresh token goes to `<data>/secrets/assistant.json` under `google:gemini:<user-id>`, so backups keep excluding it.
- **API:** `GET`/`PUT /api/v1/settings/assistant/google`, `POST /api/v1/me/assistant/google/start`, `GET /api/v1/assistant/google/callback`, `GET`/`DELETE /api/v1/me/assistant/google`; new fields on the existing assistant status readings.
- **CLI, MCP, CalDAV, vault:** none.
- **Outbound:** two more fixed Google hosts, reached only after a person connects (`accounts.google.com`, `oauth2.googleapis.com`) plus the existing `generativelanguage.googleapis.com`; AGENTS rule 5 holds.
- **Binary and bundle size:** about 8 KB of Go, no dependency; under 2 KB gzip in the lazy Settings chunk. The owner's three fields and the member's Connect/Disconnect row live in Settings.
