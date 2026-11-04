-- assistant-gemini-oauth: a person's own Google account as the credential of
-- the Gemini provider (design D6). Server state outside the op log, like
-- consent, usage and member choices. The refresh token lives in
-- <data>/secrets/assistant.json under google:gemini:<user-id>; this row holds
-- only what the screen shows.
CREATE TABLE IF NOT EXISTS assistant_google_connection (
  user_id      TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  -- account is the connected Google address, shown so a person can tell
  -- which account a turn would use. Never a token.
  account      TEXT NOT NULL DEFAULT '',
  -- project is the Google Cloud project the calls are billed to, sent as
  -- x-goog-user-project; empty falls back to the owner's project.
  project      TEXT NOT NULL DEFAULT '',
  connected_at INTEGER NOT NULL,
  -- last_error is "reconnect" when Google refused the grant, else empty.
  last_error   TEXT NOT NULL DEFAULT ''
);

-- assistant_oauth_state is one pending sign-in: single use, bound to the
-- person who started it, and stored as a hash so a database dump cannot be
-- replayed against Google (design D3).
CREATE TABLE IF NOT EXISTS assistant_oauth_state (
  state_hash   TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL,
  verifier     TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  -- project is the Google Cloud project the person typed on the connect
  -- screen, kept on the pending sign-in until the callback stores it.
  project      TEXT NOT NULL DEFAULT '',
  expires_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS assistant_oauth_state_expires ON assistant_oauth_state(expires_at);
