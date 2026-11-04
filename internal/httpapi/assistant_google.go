package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/OpenTodoApp/opentodo/internal/assistant"
	"github.com/OpenTodoApp/opentodo/internal/audit"
	"github.com/OpenTodoApp/opentodo/internal/googleoauth"
)

// A person's own Google account as the credential of the Gemini provider
// (OpenSpec change assistant-gemini-oauth). The owner configures one OAuth
// client; every account connects its own Google account and may remove it.
// The refresh token never leaves the secrets file and no access token is ever
// returned by the API.

const (
	// assistantPanel is where a finished sign-in lands.
	assistantPanel = "/settings/integrations/assistant"
	// assistantGoogleCallback is the redirect address to register at Google.
	assistantGoogleCallback = "/api/v1/assistant/google/callback"
	// googleStateNamespace keeps assistant sign-ins apart from any other
	// feature's states in the shared hashing helper.
	googleStateNamespace = "assistant"
)

func (s *Server) registerAssistantGoogleRoutes(mux *routes) {
	mux.handle("GET /api/v1/settings/assistant/google", ownerOnly("manage the assistant settings").DemoDenied(), s.handleGetAssistantGoogleClient)
	mux.handle("PUT /api/v1/settings/assistant/google", ownerOnly("manage the assistant settings").Credential().DemoDenied(), s.handlePutAssistantGoogleClient)
	mux.handle("POST /api/v1/me/assistant/google/start", signedIn.Recent().DemoDenied(), s.handleAssistantGoogleStart)
	mux.handle("GET "+assistantGoogleCallback, signedIn.SessionOnly().DemoDenied(), s.handleAssistantGoogleCallback)
	mux.handle("GET /api/v1/me/assistant/google", signedIn.DemoDenied(), s.handleGetMyGoogleConnection)
	mux.handle("DELETE /api/v1/me/assistant/google", signedIn.DemoDenied(), s.handleDeleteMyGoogleConnection)
}

// googleClientView is the owner's OAuth client as the API serves it: never the
// secret, plus the address to register at Google.
func googleClientView(c googleoauth.Client, redirectURI string) map[string]any {
	return map[string]any{
		"client_id":    c.ID,
		"secret_set":   c.Secret != "",
		"project":      c.Project,
		"complete":     c.Complete(),
		"redirect_uri": redirectURI,
	}
}

func (s *Server) assistantGoogleRedirectURI(r *http.Request) string {
	return s.publicOrigin(r) + assistantGoogleCallback
}

func (s *Server) handleGetAssistantGoogleClient(w http.ResponseWriter, r *http.Request) {
	c, err := s.assistant.Store.GoogleClient(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, googleClientView(c, s.assistantGoogleRedirectURI(r)))
}

// handlePutAssistantGoogleClient saves the owner's client. A nil secret keeps
// the stored one; an empty one removes it. Every value is checked like a save
// of the assistant's own settings, and the secret is only ever reported as set.
func (s *Server) handlePutAssistantGoogleClient(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID     *string `json:"client_id"`
		ClientSecret *string `json:"client_secret"`
		Project      *string `json:"project"`
	}
	if err := readJSON(r, &in); err != nil || (in.ClientID == nil && in.Project == nil && in.ClientSecret == nil) {
		badBodyWith(w, err, "invalid_input", `body must be {"client_id": "...", "client_secret": "...", "project": "..."}; a missing field keeps its value`)
		return
	}
	cur, err := s.assistant.Store.GoogleClient(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	next := cur
	if in.ClientID != nil {
		next.ID = *in.ClientID
	}
	if in.Project != nil {
		next.Project = *in.Project
	}
	saved, err := s.assistant.Store.SaveGoogleClient(r.Context(), userFrom(r.Context()).ID, next, in.ClientSecret, s.audit)
	var inv *assistant.InvalidError
	if errors.As(err, &inv) {
		writeError(w, http.StatusBadRequest, "invalid_input", inv.Error())
		return
	}
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, googleClientView(saved, s.assistantGoogleRedirectURI(r)))
}

// handleAssistantGoogleStart mints a state and a PKCE verifier, stores them
// for this person and answers Google's consent address.
func (s *Server) handleAssistantGoogleStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Project string `json:"project"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &in); err != nil {
			badBodyWith(w, err, "invalid_input", `body must be {"project"?: "my-project-123456"}`)
			return
		}
	}
	client, err := s.assistant.Store.GoogleClient(ctx)
	if err != nil {
		s.internal(w, err)
		return
	}
	if !client.Complete() {
		writeError(w, http.StatusConflict, "google_not_configured", "the server owner has not set up a Google client for the assistant")
		return
	}
	project := strings.TrimSpace(in.Project)
	if err := assistant.ValidateGoogleProject(project); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	state := googleoauth.NewState()
	verifier, challenge := googleoauth.PKCE()
	redirect := s.assistantGoogleRedirectURI(r)
	if err := s.assistant.Store.PutOAuthState(ctx, googleoauth.StateHash(googleStateNamespace, state),
		assistant.OAuthState{UserID: userFrom(ctx).ID, Verifier: verifier, RedirectURI: redirect, Project: project},
		assistant.OAuthStateTTL); err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": googleoauth.AuthorizationURL("", googleoauth.AuthRequest{
		ClientID: client.ID, RedirectURI: redirect, State: state, Challenge: challenge, Scopes: assistant.GoogleScopes,
	})})
}

// handleAssistantGoogleCallback finishes a sign-in started by this session and
// redirects to Settings with the outcome. Nothing is stored when the state is
// not the caller's own, when Google reports a refusal, or when the exchange
// does not return a refresh token for an account that has none.
func (s *Server) handleAssistantGoogleCallback(w http.ResponseWriter, r *http.Request) {
	back := func(result string) {
		q := url.Values{"assistant-google": {result}}
		http.Redirect(w, r, assistantPanel+"?"+q.Encode(), http.StatusSeeOther)
	}
	ctx := r.Context()
	uid := userFrom(ctx).ID
	q := r.URL.Query()
	st, err := s.assistant.Store.TakeOAuthState(ctx, googleoauth.StateHash(googleStateNamespace, q.Get("state")))
	if err != nil || st.UserID != uid {
		back("invalid_state")
		return
	}
	if q.Get("error") != "" || q.Get("code") == "" {
		back("denied")
		return
	}
	client, err := s.assistant.Store.GoogleClient(ctx)
	if err != nil || !client.Complete() {
		back("not_configured")
		return
	}
	tok, err := s.assistant.Store.GoogleTokens().Endpoint.Exchange(ctx, client, q.Get("code"), st.Verifier, st.RedirectURI)
	if err != nil {
		s.log.Warn("assistant google sign-in refused", "err", err)
		back("error")
		return
	}
	refresh := tok.Refresh
	if refresh == "" {
		// A second consent for an account that already granted the scope may
		// return no refresh token; keep the one we hold.
		sec, serr := s.assistant.Store.Secrets()
		if serr != nil {
			s.internal(w, serr)
			return
		}
		refresh = sec.Value(assistant.GoogleRefreshKey(uid))
	}
	if refresh == "" {
		back("error")
		return
	}
	if err := s.assistant.Store.SaveGoogleConnection(ctx, uid, googleoauth.Email(tok.IDToken), st.Project, refresh); err != nil {
		var inv *assistant.InvalidError
		if errors.As(err, &inv) {
			back("invalid_project")
			return
		}
		s.internal(w, err)
		return
	}
	s.assistant.Store.GoogleTokens().Cache.Put(uid, *tok)
	s.audit.Record(ctx, audit.AssistantConnectionCreated(uid, assistant.Gemini))
	back("connected")
}

// myGoogleView is one person's connection as the API serves it: presence, the
// address to show and the reconnect state, never a token.
func (s *Server) myGoogleView(r *http.Request) (map[string]any, error) {
	ctx := r.Context()
	uid := userFrom(ctx).ID
	set := s.assistantSettings(ctx)
	conn, err := s.assistant.Store.GoogleConnection(ctx, uid)
	if err != nil {
		return nil, err
	}
	client, err := s.assistant.Store.GoogleClient(ctx)
	if err != nil {
		return nil, err
	}
	view := map[string]any{
		"available":       set.AllowMemberKeys && client.Complete(),
		"connected":       conn != nil,
		"account":         "",
		"project":         "",
		"needs_reconnect": false,
	}
	if conn != nil {
		view["account"] = conn.Account
		view["project"] = conn.Project
		view["needs_reconnect"] = conn.NeedsReconnect()
	}
	return view, nil
}

func (s *Server) handleGetMyGoogleConnection(w http.ResponseWriter, r *http.Request) {
	v, err := s.myGoogleView(r)
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleDeleteMyGoogleConnection removes the stored refresh token and the row;
// nothing at Google is revoked, so the screen says the person may also remove
// OpenTodo's access there.
func (s *Server) handleDeleteMyGoogleConnection(w http.ResponseWriter, r *http.Request) {
	uid := userFrom(r.Context()).ID
	conn, err := s.assistant.Store.GoogleConnection(r.Context(), uid)
	if err != nil {
		s.internal(w, err)
		return
	}
	if conn != nil {
		if err := s.assistant.Store.DeleteGoogleConnection(r.Context(), uid); err != nil {
			s.internal(w, err)
			return
		}
		s.assistant.Store.GoogleTokens().Cache.Drop(uid)
		s.audit.Record(r.Context(), audit.AssistantConnectionDeleted(uid, assistant.Gemini))
	}
	v, err := s.myGoogleView(r)
	if err != nil {
		s.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
