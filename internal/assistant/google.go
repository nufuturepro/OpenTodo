package assistant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/OpenTodoApp/opentodo/internal/audit"
	"github.com/OpenTodoApp/opentodo/internal/googleoauth"
	"github.com/OpenTodoApp/opentodo/internal/textutil"
)

// A person's own Google account as the credential of the Gemini provider
// (OpenSpec change assistant-gemini-oauth). The provider stays `gemini`: only
// the credential changes, so consent, the disclosure, the monthly cap and the
// usage rows keep their meaning. The refresh token lives in the secrets file,
// the row holds what the screen shows, and access tokens never leave memory.

// Owner settings keys of the OAuth client and the secrets entry of its secret.
const (
	GoogleClientIDKey = "assistant.google_client_id"
	GoogleProjectKey  = "assistant.google_project"
	GoogleSecretKey   = "assistant.google_client_secret"
)

// GoogleScopes are the scopes the connect flow asks for: who the person is
// (to show the connected address) and the narrow Gemini read/serve scope that
// generation uses. If Google refuses generation for it, cloud-platform joins
// this list in one place (design D2).
var GoogleScopes = []string{
	"openid",
	"email",
	"https://www.googleapis.com/auth/generative-language.retriever",
}

// GoogleRefreshKey is a person's refresh token in the secrets file.
func GoogleRefreshKey(userID string) string { return "google:gemini:" + userID }

// GoogleReconnect is the last_error value of a connection Google refused:
// only connecting again helps.
const GoogleReconnect = "reconnect"

// ReconnectNeedsMessage is what a person sees when their grant was revoked.
const ReconnectNeedsMessage = "Google refused your account's connection; reconnect your Google account in Settings → Integrations → Assistant"

// ReconnectNeeded is the provider_auth message for a turn on a revoked connection.
func ReconnectNeeded() string { return ReconnectNeedsMessage }

// ErrNoGoogleConnection means the route needs a Google account and none is
// connected (distinct from ErrNoKey: the fix is connecting, not a key).
var ErrNoGoogleConnection = errors.New("google_not_connected")

var projectRe = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)

// ValidateGoogleProject checks an optional Cloud project id: empty means the
// client's own project.
func ValidateGoogleProject(project string) error {
	if project == "" || projectRe.MatchString(project) {
		return nil
	}
	return invalid("project", "must be a Google Cloud project id, such as my-project-123456")
}

// ---- the owner's client ----

// GoogleClient resolves the owner's OAuth client: the settings table for the
// id and the project, the secrets file for the secret.
func (st *Store) GoogleClient(ctx context.Context) (googleoauth.Client, error) {
	var c googleoauth.Client
	if _, err := st.set.Get(ctx, GoogleClientIDKey, &c.ID); err != nil {
		return c, err
	}
	if _, err := st.set.Get(ctx, GoogleProjectKey, &c.Project); err != nil {
		return c, err
	}
	sec, err := st.secrets.Load()
	c.Secret = sec.Value(GoogleSecretKey)
	return c, err
}

// SaveGoogleClient stores the owner's client. A nil secret keeps the stored
// one, an empty one removes it (never sent back by the API). It records
// settings.changed (assistant.google_client, unset/set), never the values.
func (st *Store) SaveGoogleClient(ctx context.Context, actorID string, c googleoauth.Client, secret *string, rec *audit.Recorder) (googleoauth.Client, error) {
	c.ID = strings.TrimSpace(c.ID)
	c.Project = strings.TrimSpace(c.Project)
	if len(c.ID) > maxGoogleField || (secret != nil && len(*secret) > maxGoogleField) {
		return googleoauth.Client{}, invalid("client_id", "the client id and secret are at most 1024 characters")
	}
	if strings.ContainsAny(c.ID, " \r\n\t") {
		return googleoauth.Client{}, invalid("client_id", "must be the client id Google shows, on one line")
	}
	if c.Project != "" && !projectRe.MatchString(c.Project) {
		return googleoauth.Client{}, invalid("project", "must be a Google Cloud project id, such as my-project-123456")
	}
	before, err := st.GoogleClient(ctx)
	if err != nil {
		return googleoauth.Client{}, err
	}
	if err := st.set.Set(ctx, GoogleClientIDKey, c.ID); err != nil {
		return googleoauth.Client{}, err
	}
	if err := st.set.Set(ctx, GoogleProjectKey, c.Project); err != nil {
		return googleoauth.Client{}, err
	}
	if secret != nil {
		v := strings.TrimSpace(*secret)
		if err := st.secrets.Update(map[string]*string{GoogleSecretKey: &v}); err != nil {
			return googleoauth.Client{}, err
		}
	}
	after, err := st.GoogleClient(ctx)
	if err != nil {
		return googleoauth.Client{}, err
	}
	if rec != nil && (before.Complete() != after.Complete() || before.Project != after.Project) {
		rec.Record(ctx, audit.SettingsChanged(actorID, "assistant.google_client", presence(before.Complete()), presence(after.Complete())))
	}
	return after, nil
}

// maxGoogleField bounds the client id and secret.
const maxGoogleField = 1024

// ---- a person's connection ----

// GoogleConnection is one person's connected Google account, as the API shows
// it. It never carries a token.
type GoogleConnection struct {
	// Account is the connected Google address, for display.
	Account     string `json:"account"`
	Project     string `json:"project"`
	ConnectedAt int64  `json:"connected_at"`
	LastError   string `json:"last_error,omitempty"`
}

// NeedsReconnect reports a connection Google refused.
func (c *GoogleConnection) NeedsReconnect() bool { return c != nil && c.LastError == GoogleReconnect }

// GoogleConnection returns the person's connection, or nil.
func (st *Store) GoogleConnection(ctx context.Context, userID string) (*GoogleConnection, error) {
	var c GoogleConnection
	err := st.db.QueryRowContext(ctx, `SELECT account, project, connected_at, last_error FROM assistant_google_connection WHERE user_id = ?`, userID).
		Scan(&c.Account, &c.Project, &c.ConnectedAt, &c.LastError)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read assistant google connection: %w", err)
	}
	return &c, nil
}

// HasGoogleConnection reports whether the person connected a Google account.
func (st *Store) HasGoogleConnection(ctx context.Context, userID string) (bool, error) {
	c, err := st.GoogleConnection(ctx, userID)
	return c != nil, err
}

// SaveGoogleConnection stores the refresh token and the row after a completed
// sign-in, replacing an earlier connection of the same person.
func (st *Store) SaveGoogleConnection(ctx context.Context, userID, account, project, refresh string) error {
	if refresh == "" {
		return errors.New("assistant: a google sign-in returned no refresh token")
	}
	project = strings.TrimSpace(project)
	if err := ValidateGoogleProject(project); err != nil {
		return err
	}
	token := refresh
	if err := st.secrets.Update(map[string]*string{GoogleRefreshKey(userID): &token}); err != nil {
		return err
	}
	_, err := st.db.ExecContext(ctx, `INSERT INTO assistant_google_connection(user_id, account, project, connected_at, last_error) VALUES(?,?,?,?,'')
		ON CONFLICT(user_id) DO UPDATE SET account = excluded.account, project = excluded.project,
		connected_at = excluded.connected_at, last_error = ''`,
		userID, textutil.Truncate(account, 200), project, st.now().UnixMilli())
	if err != nil {
		return fmt.Errorf("store assistant google connection: %w", err)
	}
	return nil
}

// DeleteGoogleConnection removes the refresh token and the row.
func (st *Store) DeleteGoogleConnection(ctx context.Context, userID string) error {
	empty := ""
	if err := st.secrets.Update(map[string]*string{GoogleRefreshKey(userID): &empty}); err != nil {
		return err
	}
	_, err := st.db.ExecContext(ctx, `DELETE FROM assistant_google_connection WHERE user_id = ?`, userID)
	return err
}

// MarkGoogleReconnect records that Google refused the grant, so the screen
// offers Reconnect instead of a silent failure.
func (st *Store) MarkGoogleReconnect(ctx context.Context, userID string) error {
	_, err := st.db.ExecContext(ctx, `UPDATE assistant_google_connection SET last_error = ? WHERE user_id = ?`, GoogleReconnect, userID)
	return err
}

// ---- pending sign-ins ----

// OAuthState is one pending sign-in (single use, bound to its person).
type OAuthState struct {
	UserID      string
	Verifier    string
	RedirectURI string
	// Project is the Cloud project the person typed, stored with the
	// connection once the callback completes.
	Project string
}

// OAuthStateTTL bounds a pending sign-in.
const OAuthStateTTL = 10 * time.Minute

// PutOAuthState stores a pending sign-in under the hash of its state.
func (st *Store) PutOAuthState(ctx context.Context, stateHash string, s OAuthState, ttl time.Duration) error {
	now := st.now()
	if _, err := st.db.ExecContext(ctx, `DELETE FROM assistant_oauth_state WHERE expires_at < ?`, now.UnixMilli()); err != nil {
		return err
	}
	_, err := st.db.ExecContext(ctx, `INSERT INTO assistant_oauth_state(state_hash, user_id, verifier, redirect_uri, project, expires_at) VALUES(?,?,?,?,?,?)`,
		stateHash, s.UserID, s.Verifier, s.RedirectURI, s.Project, now.Add(ttl).UnixMilli())
	return err
}

// TakeOAuthState removes and returns a pending sign-in that has not expired.
func (st *Store) TakeOAuthState(ctx context.Context, stateHash string) (*OAuthState, error) {
	var s OAuthState
	var expires int64
	err := st.db.QueryRowContext(ctx, `SELECT user_id, verifier, redirect_uri, project, expires_at FROM assistant_oauth_state WHERE state_hash = ?`, stateHash).
		Scan(&s.UserID, &s.Verifier, &s.RedirectURI, &s.Project, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("invalid_state")
	}
	if err != nil {
		return nil, fmt.Errorf("read assistant oauth state: %w", err)
	}
	if _, err := st.db.ExecContext(ctx, `DELETE FROM assistant_oauth_state WHERE state_hash = ?`, stateHash); err != nil {
		return nil, err
	}
	if st.now().UnixMilli() > expires {
		return nil, errors.New("invalid_state")
	}
	return &s, nil
}

// ---- access tokens ----

// GoogleTokens is the store's token minter (tests point its endpoint at a
// fake Google token service).
func (st *Store) GoogleTokens() *GoogleAuth { return st.google }

// GoogleAuth mints access tokens of connected accounts: one refresh call per
// hour per person, the token in memory only.
type GoogleAuth struct {
	Store *Store
	// Endpoint is Google's token endpoint (tests point it at a fake).
	Endpoint googleoauth.Endpoint
	Cache    googleoauth.Cache
}

// TokenSource returns the bearer-token source of one person's connection, or
// nil when they have not connected. Minting is lazy: a turn that never calls
// the provider never talks to Google.
func (g *GoogleAuth) TokenSource(userID string) TokenSource {
	return &googleToken{ga: g, userID: userID}
}

// AccessToken is the cached or freshly minted access token of userID.
func (g *GoogleAuth) AccessToken(ctx context.Context, userID string) (string, error) {
	if t, ok := g.Cache.Get(userID); ok {
		return t, nil
	}
	return g.mint(ctx, userID)
}

// Refresh drops the cached token and mints a new one (after a 401).
func (g *GoogleAuth) Refresh(ctx context.Context, userID string) (string, error) {
	g.Cache.Drop(userID)
	return g.mint(ctx, userID)
}

// mint exchanges the stored refresh token for an access token. A refused
// grant marks the connection and answers a provider_auth the UI words as
// Reconnect; it never echoes the token or Google's description.
func (g *GoogleAuth) mint(ctx context.Context, userID string) (string, error) {
	sec, err := g.Store.Secrets()
	if err != nil {
		return "", &ProviderError{Code: CodeProviderAuth, Msg: "the assistant's secrets file could not be read"}
	}
	refresh := sec.Value(GoogleRefreshKey(userID))
	if refresh == "" {
		return "", ErrNoGoogleConnection
	}
	client, err := g.Store.GoogleClient(ctx)
	if err != nil {
		return "", &ProviderError{Code: CodeProviderAuth, Msg: "the assistant's Google client could not be read"}
	}
	if !client.Complete() {
		return "", &ProviderError{Code: CodeProviderAuth, Msg: "the server owner has not set up a Google client for the assistant"}
	}
	tok, err := g.Endpoint.Refresh(ctx, client, refresh)
	if err != nil {
		var oe *googleoauth.Error
		if errors.As(err, &oe) && oe.Code == googleoauth.CodeUnauthorized {
			if markErr := g.Store.MarkGoogleReconnect(context.WithoutCancel(ctx), userID); markErr != nil {
				return "", markErr
			}
			return "", &ProviderError{Code: CodeProviderAuth, Msg: ReconnectNeedsMessage}
		}
		if errors.As(err, &oe) && oe.Code == googleoauth.CodeRateLimited {
			return "", &ProviderError{Code: CodeProviderRateLimited, Msg: "Google is rate limiting the sign-in; try again in a moment"}
		}
		return "", &ProviderError{Code: CodeProviderError, Msg: "Google's sign-in service could not be reached"}
	}
	g.Cache.Put(userID, *tok)
	return tok.Access, nil
}

// TokenSource is the bearer-token source one provider request uses.
type TokenSource interface {
	// Token is the access token, minted when the cached one expired.
	Token(ctx context.Context) (string, error)
	// Refresh drops the cached token and mints a new one after a 401.
	Refresh(ctx context.Context) (string, error)
}

type googleToken struct {
	ga     *GoogleAuth
	userID string
}

func (t *googleToken) Token(ctx context.Context) (string, error) {
	return t.ga.AccessToken(ctx, t.userID)
}

func (t *googleToken) Refresh(ctx context.Context) (string, error) {
	return t.ga.Refresh(ctx, t.userID)
}

// googleFor is the credential of a turn on the person's own Google account,
// when they connected one and the provider is Gemini. ok is false when there
// is nothing to use, including an owner whose OAuth client was removed since:
// the turn then falls back to the operator's credential (design D2).
func (st *Store) googleFor(ctx context.Context, userID, provider string) (tok TokenSource, project string, ok bool, err error) {
	if provider != Gemini {
		return nil, "", false, nil
	}
	conn, err := st.GoogleConnection(ctx, userID)
	if err != nil || conn == nil {
		return nil, "", false, err
	}
	client, err := st.GoogleClient(ctx)
	if err != nil {
		return nil, "", false, err
	}
	if !client.Complete() {
		return nil, "", false, nil
	}
	project = conn.Project
	if project == "" {
		project = client.Project
	}
	return st.google.TokenSource(userID), project, true, nil
}
