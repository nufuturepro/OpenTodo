## ADDED Requirements

### Requirement: A Google account is a credential for Gemini

The server SHALL let a person use Google Gemini with their own Google account instead of an API key. The provider SHALL stay `gemini` and its model SHALL stay the owner's or the member's choice, so consent, the disclosure, the monthly cap and usage rows are unchanged. A turn on a connection SHALL send the access token as `Authorization: Bearer` and SHALL count as an own turn for `cap_member_keys` and the own-key usage row.

The order of credentials for a turn SHALL be: the member's own provider with an own API key; the member's own provider with a Google connection; the owner's default provider with the person's own Google connection when `allow_member_keys` is on; otherwise the operator's key. An own API key SHALL win over a connection, and the Gemini card SHALL say so.

#### Scenario: A turn on a connection

- **WHEN** a person who connected their Google account sends a turn while outbound traffic is inspected
- **THEN** the request to `generativelanguage.googleapis.com` carries `Authorization: Bearer <access token>` and no API key, and the turn is recorded as an own turn

#### Scenario: Own key still wins

- **WHEN** a person has both an own API key and a Google connection for Gemini and sends a turn
- **THEN** the request carries the API key, and Settings says that the key is in use

#### Scenario: Owner's provider with the person's connection

- **WHEN** the owner's default provider is Gemini with no operator key, member keys are allowed, and a member connected their Google account
- **THEN** that member's turn runs on their own connection and leaves the instance's other members unaffected

#### Scenario: Connection but no permission

- **WHEN** a person connected a Google account while `allow_member_keys` is off
- **THEN** their turns use the operator's credential, and re-enabling the switch makes the connection effective again without reconnecting

### Requirement: The owner configures one OAuth client

The owner SHALL set a Google OAuth client through `GET` and `PUT /api/v1/settings/assistant/google`: `client_id`, `client_secret` and an optional `project`, with the redirect address to register at Google returned for display. The secret SHALL be stored only in the secrets file, SHALL never be returned by the API, logged or exported, and SHALL be audited as `settings.changed` for `assistant.google_client` with `unset` or `set`, never the value. This client SHALL be separate from calendar sync's Google client. While the client is incomplete, connecting SHALL be refused as `google_not_configured` and no turn SHALL use a connection.

#### Scenario: The secret never comes back

- **WHEN** the owner saves a client id and secret and then reads the settings and the audit log
- **THEN** the response reports `client_set: true`, the audit event reports `set`, and neither contains the secret

#### Scenario: A member cannot change the client

- **WHEN** a member calls `PUT /api/v1/settings/assistant/google`
- **THEN** the response is 403 `forbidden` and nothing changes

#### Scenario: Anonymous request

- **WHEN** a request without credentials calls either client route
- **THEN** the response is 401 `unauthorized`

#### Scenario: Incomplete client

- **WHEN** a person calls the start route before the owner saved a client id and secret
- **THEN** the response is 409 `google_not_configured` and no request reaches Google

### Requirement: Connect and disconnect a Google account

`POST /api/v1/me/assistant/google/start` SHALL answer a Google consent URL carrying a single-use state and a PKCE S256 challenge, stored hashed for at most ten minutes and bound to the caller. `GET /api/v1/assistant/google/callback` SHALL accept only the caller's own unexpired state, exchange the code at Google's token endpoint and store the refresh token in the secrets file under `google:gemini:<user-id>`. It SHALL require a session (bearer tokens get 403 `session_required`). `GET /api/v1/me/assistant/google` SHALL report whether a connection exists and the connected address; `DELETE` SHALL remove the stored token and the row. Every route SHALL be denied on a demo instance, and the callback SHALL redirect to Settings with `assistant-google=connected`, `denied` or `invalid_state`.

#### Scenario: Connecting

- **WHEN** a person accepts at Google and returns to the callback with the state they were given
- **THEN** a connection exists with the account's address, and `GET /api/v1/me/assistant` reports `google_connected: true` and that address

#### Scenario: Declining

- **WHEN** the person declines the consent screen
- **THEN** the callback redirects with `assistant-google=denied` and no connection or secret is stored

#### Scenario: Another person's state

- **WHEN** a callback arrives with a state minted for another account, or an expired one
- **THEN** the redirect says `invalid_state`, no token is exchanged and nothing is stored

#### Scenario: Disconnecting

- **WHEN** a person disconnects
- **THEN** the refresh token is deleted from the secrets file, the row is gone, and their next turn uses the remaining credential or answers `no_key`

#### Scenario: Not signed in

- **WHEN** a request without a session calls the start route or the callback
- **THEN** the response is 401 `unauthorized` and no request reaches Google

#### Scenario: Account deleted

- **WHEN** an account with a connection is deleted
- **THEN** its refresh token and its row are removed with the account

### Requirement: Access tokens are short-lived and never stored

The server SHALL mint access tokens from the stored refresh token on demand, SHALL keep them in memory only, and SHALL treat them as expiring after the lifetime Google reports. A 401 SHALL drop the cached token and retry once with a freshly minted one. A refresh refused by Google SHALL mark the connection as needing a reconnect and SHALL answer `provider_auth` with a message naming the Google account, not the API key. No token SHALL appear in a log line, an error message, the audit log or an export.

#### Scenario: Expired token

- **WHEN** a cached access token has expired and the person sends a turn
- **THEN** the server refreshes once before the request and the turn succeeds

#### Scenario: Token refused

- **WHEN** the provider answers 401 for a minted token and the fresh one is accepted
- **THEN** the turn succeeds and the refusal is not reported to the person

#### Scenario: Refresh revoked

- **WHEN** the person revokes OpenTodo's access in their Google account and sends a turn
- **THEN** the connection is marked as needing a reconnect, the turn ends with `provider_auth` saying to reconnect, and Settings offers Reconnect

#### Scenario: No token in logs

- **WHEN** a turn, a test or a refresh fails
- **THEN** the log line, the error and the audit entry name the provider and the outcome only

### Requirement: Connection state errors

The connection's routes and turns SHALL report: 409 `google_not_configured` when the owner has no complete client, 409 `google_not_connected` when a turn needs a connection and there is none, 400 `invalid_state` for an unknown or expired sign-in, and 403 `google_denied` when the person declined. Each SHALL be denied on a demo instance.

#### Scenario: Demo instance

- **WHEN** any of the connection routes is called on a demo instance
- **THEN** the response is 404 `not_found` and nothing is stored

#### Scenario: Turn with no credential

- **WHEN** the owner enabled Gemini expecting connections but nobody has connected and no operator key exists
- **THEN** the turn is refused with `google_not_connected` and its message says to connect a Google account or ask the owner for a key
