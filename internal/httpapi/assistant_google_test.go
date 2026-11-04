package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/OpenTodoApp/opentodo/internal/assistant"
	"github.com/OpenTodoApp/opentodo/internal/googleoauth"
)

// The Gemini provider on a person's own Google account (assistant-gemini-oauth)
// through the real HTTP stack, against fakes of Google's token endpoint and of
// the Gemini API. No test contacts Google.

type googleFakes struct {
	tokens *httptest.Server
	chat   *httptest.Server
	mu     sync.Mutex
	auth   []string
	proj   []string
}

func (f *googleFakes) headers() ([]string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.auth...), append([]string(nil), f.proj...)
}

func newGoogleFakes(t *testing.T) *googleFakes {
	t.Helper()
	f := &googleFakes{}
	f.tokens = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		claims, _ := json.Marshal(map[string]any{"email": "ana@example.com", "email_verified": true})
		id := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString(claims) + ".sig"
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			fmt.Fprintf(w, `{"access_token":"access-connected","refresh_token":"refresh-1","id_token":%q,"expires_in":3600}`, id)
		case "refresh_token":
			fmt.Fprint(w, `{"access_token":"access-refreshed","expires_in":3600}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"unsupported_grant_type"}`)
		}
	}))
	t.Cleanup(f.tokens.Close)
	f.chat = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.proj = append(f.proj, r.Header.Get("x-goog-user-project"))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Nothing is due today.\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(f.chat.Close)
	return f
}

// googleEnv is an owner whose server reaches the fakes.
func googleEnv(t *testing.T) (*env, *googleFakes, **assistant.Service) {
	t.Helper()
	fakes := newGoogleFakes(t)
	var svc *assistant.Service
	e := newEnvWith(t, true, WithAssistantSetup(func(s *assistant.Service) {
		s.GeminiBaseURL = fakes.chat.URL
		s.Fixed = fakes.chat.Client()
		s.Store.GoogleTokens().Endpoint = googleoauth.Endpoint{HTTP: fakes.tokens.Client(), URL: fakes.tokens.URL}
		svc = s
	}))
	resp, out := e.do("POST", "/api/v1/auth/register", map[string]string{"email": "owner@example.com", "name": "Owner", "password": "correct horse battery"})
	e.mustStatus(resp, 201, out)
	return e, fakes, &svc
}

// saveGoogleClient sets the owner's client through the route.
func (e *env) saveGoogleClient(project string) map[string]any {
	e.t.Helper()
	resp, out := e.do("PUT", "/api/v1/settings/assistant/google", map[string]any{"client_id": "client-1.apps.googleusercontent.com", "client_secret": "GOCSPX-secret", "project": project})
	e.mustStatus(resp, 200, out)
	return out
}

// connectGoogle signs in with a code the fake exchanges.
func (e *env) connectGoogle() string {
	e.t.Helper()
	resp, out := e.do("POST", "/api/v1/me/assistant/google/start", map[string]any{"project": "my-project-123456"})
	e.mustStatus(resp, 200, out)
	raw, _ := out["url"].(string)
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(raw, googleoauth.AuthURL) {
		e.t.Fatalf("start url: %q %v", raw, err)
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		e.t.Fatalf("no PKCE: %s", raw)
	}
	resp, _ = e.do("GET", "/api/v1/assistant/google/callback?state="+url.QueryEscape(u.Query().Get("state"))+"&code=code-1", nil)
	if got := resp.Request.URL.Query().Get("assistant-google"); got != "connected" {
		e.t.Fatalf("callback landed on %s", resp.Request.URL)
	}
	return u.Query().Get("state")
}

func TestAssistantGoogleRoutesRequireTheirPolicy(t *testing.T) {
	e, _, _ := googleEnv(t)
	anon := newClientFor(e)
	resp, out := anon.do("GET", "/api/v1/settings/assistant/google", nil)
	if resp.StatusCode != 401 || errCode(out) != "unauthorized" {
		t.Fatalf("anonymous settings: %d %v", resp.StatusCode, out)
	}
	resp, out = anon.do("POST", "/api/v1/me/assistant/google/start", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous start: %d %v", resp.StatusCode, out)
	}
	// A member may connect their own account but never the owner's client.
	member := e.memberClient("member@example.com")
	resp, out = member.do("PUT", "/api/v1/settings/assistant/google", map[string]any{"client_id": "x"})
	if resp.StatusCode != 403 || errCode(out) != "forbidden" {
		t.Fatalf("member settings: %d %v", resp.StatusCode, out)
	}
	e.saveGoogleClient("")
	resp, out = e.do("GET", "/api/v1/settings/assistant/google", nil)
	e.mustStatus(resp, 200, out)
	if out["secret_set"] != true || out["complete"] != true {
		t.Fatalf("client view: %v", out)
	}
	if strings.Contains(fmt.Sprint(out), "GOCSPX-secret") {
		t.Fatalf("the secret came back: %v", out)
	}
	if !strings.HasSuffix(out["redirect_uri"].(string), "/api/v1/assistant/google/callback") {
		t.Fatalf("redirect address: %v", out["redirect_uri"])
	}
}

func TestAssistantGoogleStartNeedsTheOwnerClient(t *testing.T) {
	e, _, _ := googleEnv(t)
	resp, out := e.do("POST", "/api/v1/me/assistant/google/start", nil)
	if resp.StatusCode != 409 || errCode(out) != "google_not_configured" {
		t.Fatalf("start without a client: %d %v", resp.StatusCode, out)
	}
	e.saveGoogleClient("")
	// A project id Google would refuse is refused before the sign-in starts.
	resp, out = e.do("POST", "/api/v1/me/assistant/google/start", map[string]any{"project": "Not A Project"})
	if resp.StatusCode != 409 && resp.StatusCode != 400 {
		t.Fatalf("bad project: %d %v", resp.StatusCode, out)
	}
}

func TestAssistantGoogleConnectAndDisconnect(t *testing.T) {
	e, _, svc := googleEnv(t)
	e.saveGoogleClient("")
	state := e.connectGoogle()

	resp, out := e.do("GET", "/api/v1/me/assistant", nil)
	e.mustStatus(resp, 200, out)
	if out["google_connected"] != true || out["google_account"] != "ana@example.com" {
		t.Fatalf("status after connecting: %v", out)
	}
	sec, err := (*svc).Store.Secrets()
	if err != nil || sec.Value(assistant.GoogleRefreshKey(userIDOf(t, e))) == "" {
		t.Fatalf("refresh token not stored: %v", err)
	}
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE name = 'assistant_connection.created'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit created: %d %v", n, err)
	}
	// The same state cannot be replayed.
	resp, _ = e.do("GET", "/api/v1/assistant/google/callback?state="+url.QueryEscape(state)+"&code=code-2", nil)
	if got := resp.Request.URL.Query().Get("assistant-google"); got != "invalid_state" {
		t.Fatalf("replayed state landed on %s", resp.Request.URL)
	}
	resp, out = e.do("DELETE", "/api/v1/me/assistant/google", nil)
	e.mustStatus(resp, 200, out)
	if out["connected"] != false {
		t.Fatalf("still connected: %v", out)
	}
	if sec, _ := (*svc).Store.Secrets(); sec.Has(assistant.GoogleRefreshKey(userIDOf(t, e))) {
		t.Fatal("the refresh token was kept")
	}
}

func TestAssistantGoogleForeignStateIsRefused(t *testing.T) {
	e, _, _ := googleEnv(t)
	e.saveGoogleClient("")
	resp, out := e.do("POST", "/api/v1/me/assistant/google/start", nil)
	e.mustStatus(resp, 200, out)
	consent, _ := url.Parse(out["url"].(string))
	// A person who never started this sign-in cannot finish it.
	member := e.memberClient("other@example.com")
	resp, _ = member.do("GET", "/api/v1/assistant/google/callback?state="+url.QueryEscape(consent.Query().Get("state"))+"&code=code-1", nil)
	if got := resp.Request.URL.Query().Get("assistant-google"); got != "invalid_state" {
		t.Fatalf("foreign state landed on %s", resp.Request.URL)
	}
}

func TestAssistantTurnOnTheGoogleAccount(t *testing.T) {
	e, fakes, _ := googleEnv(t)
	e.saveGoogleClient("")
	e.connectGoogle()
	// Gemini allowed for members with no operator key: the account is the credential.
	resp, out := e.do("PUT", "/api/v1/settings/assistant", map[string]any{"enabled": true, "allow_member_keys": true, "default_provider": "gemini",
		"providers": map[string]any{"gemini": map[string]any{"enabled": true, "model": "gemini-2.5-flash"}}})
	e.mustStatus(resp, 200, out)
	e.consentAssistant("gemini")

	code, out, events := e.turn(ask("What is due today?"))
	if code != 200 {
		t.Fatalf("turn: %d %v", code, out)
	}
	if len(named(events, "done")) != 1 {
		t.Fatalf("no done event: %v", events)
	}
	auth, proj := fakes.headers()
	if len(auth) == 0 || !strings.HasPrefix(auth[0], "Bearer access-") {
		t.Fatalf("the turn did not send the account's token: %v", auth)
	}
	if len(proj) == 0 || proj[0] != "my-project-123456" {
		t.Fatalf("x-goog-user-project: %v", proj)
	}
}

// memberClient registers a second account from another address.
func (e *env) memberClient(email string) *env {
	e.t.Helper()
	c := newClientAt(e, "10.0.0.30")
	resp, out := c.do("POST", "/api/v1/auth/register", map[string]string{"email": email, "name": "Member", "password": "correct horse battery"})
	c.mustStatus(resp, 201, out)
	return c
}
