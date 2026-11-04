package googleoauth

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPKCEIsVerifierAndS256Challenge(t *testing.T) {
	verifier, challenge := PKCE()
	if len(verifier) < 43 {
		t.Fatalf("verifier too short: %q", verifier)
	}
	sum := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); challenge != want {
		t.Fatalf("challenge %q, want %q", challenge, want)
	}
	other, _ := PKCE()
	if other == verifier {
		t.Fatal("two verifiers are equal")
	}
}

func TestStateHashIsNamespaced(t *testing.T) {
	a := StateHash("assistant", "state")
	b := StateHash("calendar", "state")
	if a == b {
		t.Fatal("namespaces collide")
	}
	if a != StateHash("assistant", "state") {
		t.Fatal("the hash is not stable")
	}
	if strings.Contains(a, "state") {
		t.Fatal("the hash carries the state")
	}
}

func TestAuthorizationURLAsksForOfflineAccess(t *testing.T) {
	raw := AuthorizationURL("", AuthRequest{ClientID: "cid", RedirectURI: "https://todo.example/api/v1/assistant/google/callback",
		State: "st", Challenge: "ch", Scopes: []string{"openid", "email"}})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, AuthURL) {
		t.Fatalf("not Google's endpoint: %s", raw)
	}
	q := u.Query()
	for k, want := range map[string]string{"client_id": "cid", "state": "st", "code_challenge": "ch", "code_challenge_method": "S256",
		"access_type": "offline", "response_type": "code", "redirect_uri": "https://todo.example/api/v1/assistant/google/callback", "scope": "openid email"} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
	if q.Get("prompt") != "consent" {
		t.Error("a reconnect would not return a refresh token")
	}
	if AuthorizationURL("https://auth.example/x", AuthRequest{LoginHint: "ana@example.com"})[:len("https://auth.example/x?")] != "https://auth.example/x?" {
		t.Error("the endpoint override was ignored")
	}
}

// tokenServer answers one grant per call; fail answers Google's refusal.
type tokenServer struct {
	srv    *httptest.Server
	fail   string
	status int
	forms  []url.Values
}

func newTokenServer(t *testing.T) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		ts.forms = append(ts.forms, r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		if ts.status != 0 {
			w.WriteHeader(ts.status)
		}
		if ts.fail != "" {
			if ts.status == 0 {
				w.WriteHeader(http.StatusBadRequest)
			}
			_, _ = w.Write([]byte(`{"error":"` + ts.fail + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"a1","refresh_token":"r1","id_token":"h.p.s","expires_in":3600}`))
	}))
	t.Cleanup(ts.srv.Close)
	return ts
}

func TestExchangeAndRefresh(t *testing.T) {
	ts := newTokenServer(t)
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	e := &Endpoint{HTTP: ts.srv.Client(), URL: ts.srv.URL, Now: func() time.Time { return now }}
	c := Client{ID: "cid", Secret: "csecret"}
	tok, err := e.Exchange(t.Context(), c, "code-1", "verifier-1", "https://cb")
	if err != nil {
		t.Fatal(err)
	}
	if tok.Access != "a1" || tok.Refresh != "r1" || tok.IDToken != "h.p.s" || !tok.Expires.Equal(now.Add(time.Hour)) {
		t.Fatalf("token: %+v", tok)
	}
	form := ts.forms[0]
	for k, want := range map[string]string{"grant_type": "authorization_code", "code": "code-1", "code_verifier": "verifier-1",
		"redirect_uri": "https://cb", "client_id": "cid", "client_secret": "csecret"} {
		if form.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, form.Get(k), want)
		}
	}
	if _, err := e.Refresh(t.Context(), c, "r1"); err != nil {
		t.Fatal(err)
	}
	if got := ts.forms[1].Get("grant_type"); got != "refresh_token" {
		t.Fatalf("refresh sent %q", got)
	}
	if got := ts.forms[1].Get("refresh_token"); got != "r1" {
		t.Fatalf("refresh sent %q", got)
	}
}

func TestRefusalsAreClassified(t *testing.T) {
	for _, tc := range []struct {
		fail   string
		status int
		want   string
	}{
		{"invalid_grant", 0, CodeUnauthorized},
		{"invalid_client", 0, CodeUnauthorized},
		{"access_denied", 0, CodeUnauthorized},
		{"invalid_request", 0, CodeInvalid},
		{"server_error", http.StatusServiceUnavailable, CodeRateLimited},
	} {
		ts := newTokenServer(t)
		ts.fail, ts.status = tc.fail, tc.status
		e := &Endpoint{HTTP: ts.srv.Client(), URL: ts.srv.URL}
		_, err := e.Refresh(t.Context(), Client{ID: "c"}, "r")
		oe, ok := err.(*Error)
		if !ok || oe.Code != tc.want {
			t.Errorf("%s (%d): %v, want %s", tc.fail, tc.status, err, tc.want)
		}
	}
	// A rate-limited answer says when to try again.
	ts := newTokenServer(t)
	ts.fail, ts.status = "rate_limited", http.StatusTooManyRequests
	e := &Endpoint{HTTP: ts.srv.Client(), URL: ts.srv.URL}
	_, err := e.Refresh(t.Context(), Client{ID: "c"}, "r")
	oe, ok := err.(*Error)
	if !ok || oe.Code != CodeRateLimited {
		t.Fatalf("rate limited: %v", err)
	}
}

func TestUnreachableEndpoint(t *testing.T) {
	e := &Endpoint{HTTP: &http.Client{}, URL: "http://127.0.0.1:1/token"}
	_, err := e.Refresh(t.Context(), Client{ID: "c"}, "r")
	oe, ok := err.(*Error)
	if !ok || oe.Code != CodeUnreachable {
		t.Fatalf("unreachable: %v", err)
	}
}

func TestCacheExpiresWithSlack(t *testing.T) {
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	c := &Cache{Now: func() time.Time { return now }}
	tok := Token{Access: "a1", Expires: now.Add(time.Hour)}
	c.Put("u1", tok)
	if got, ok := c.Get("u1"); !ok || got != "a1" {
		t.Fatalf("fresh: %q %v", got, ok)
	}
	now = now.Add(59*time.Minute + 30*time.Second)
	if _, ok := c.Get("u1"); ok {
		t.Fatal("a token about to expire was served")
	}
	c.Drop("u1")
	if _, ok := c.Get("u1"); ok {
		t.Fatal("a dropped token was served")
	}
}
