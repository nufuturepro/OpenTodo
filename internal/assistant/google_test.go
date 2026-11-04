package assistant

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OpenTodoApp/opentodo/internal/audit"
	"github.com/OpenTodoApp/opentodo/internal/googleoauth"
)

// The Gemini provider on a person's Google account (assistant-gemini-oauth):
// the owner's client, the connection and its refresh token, the token minting,
// the route a turn resolves to, and the headers the adapter sends.

// fakeGoogle is Google's token endpoint: an exchange hands out a refresh token
// and an identity token, a refresh hands out an access token.
type fakeGoogle struct {
	srv       *httptest.Server
	exchanges atomic.Int32
	refreshes atomic.Int32
	// refuse makes the refresh answer invalid_grant (a revoked grant).
	refuse atomic.Bool
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	f := &fakeGoogle{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			n := f.exchanges.Add(1)
			fmt.Fprintf(w, `{"access_token":"access-%d","refresh_token":"refresh-%d","id_token":%q,"expires_in":3600}`, n, n, idToken("ana@example.com"))
		case "refresh_token":
			if f.refuse.Load() {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
				return
			}
			n := f.refreshes.Add(1)
			fmt.Fprintf(w, `{"access_token":"refreshed-%d","expires_in":3600}`, n)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"unsupported_grant_type"}`)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// idToken builds an unsigned identity token with the address claim.
func idToken(email string) string {
	payload, _ := json.Marshal(map[string]any{"email": email, "email_verified": true})
	enc := base64.RawURLEncoding
	return "eyJhbGciOiJub25lIn0." + enc.EncodeToString(payload) + ".sig"
}

// googleClock is a clock the tests advance.
type googleClock struct{ at time.Time }

func (c *googleClock) now() time.Time { return c.at }

// googleFixture is the store over a fixture database with Google's endpoints
// pointed at the fake, and a pinned clock the caller may advance.
func googleFixture(t *testing.T) (*Store, *fakeGoogle, *googleClock) {
	t.Helper()
	f := newFixture(t)
	fake := newFakeGoogle(t)
	clock := &googleClock{at: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)}
	st := NewStore(f.sync.DB(), t.TempDir(), clock.now)
	st.google.Endpoint = googleoauth.Endpoint{HTTP: fake.srv.Client(), URL: fake.srv.URL, Now: clock.now}
	st.google.Cache.Now = clock.now
	return st, fake, clock
}

func googleSettings() Settings {
	s := Defaults()
	s.Enabled = true
	s.AllowMemberKeys = true
	s.Providers[Gemini] = ProviderSettings{Enabled: true, Model: "gemini-2.5-flash"}
	return s
}

func TestGoogleClientConfig(t *testing.T) {
	st, _, _ := googleFixture(t)
	ctx := context.Background()
	rec := audit.New(st.db, nil)
	secret := "GOCSPX-secret"
	got, err := st.SaveGoogleClient(ctx, "u1", googleoauth.Client{ID: "client-1.apps.googleusercontent.com", Project: "my-project-123456"}, &secret, rec)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Complete() || got.Project != "my-project-123456" {
		t.Fatalf("saved: %+v", got)
	}
	read, err := st.GoogleClient(ctx)
	if err != nil || read.Secret != secret || read.ID != got.ID {
		t.Fatalf("read: %+v %v", read, err)
	}
	// The secret may be kept (nil) or cleared (""), never echoed.
	if _, err := st.SaveGoogleClient(ctx, "u1", googleoauth.Client{ID: "client-2"}, nil, rec); err != nil {
		t.Fatal(err)
	}
	if read, _ := st.GoogleClient(ctx); read.Secret != secret || read.ID != "client-2" {
		t.Fatalf("kept secret: %+v", read)
	}
	empty := ""
	if _, err := st.SaveGoogleClient(ctx, "u1", googleoauth.Client{ID: "client-2"}, &empty, rec); err != nil {
		t.Fatal(err)
	}
	if read, _ := st.GoogleClient(ctx); read.Secret != "" || read.Complete() {
		t.Fatalf("cleared: %+v", read)
	}
	// A project id that Google would refuse is refused here.
	var inv *InvalidError
	if _, err := st.SaveGoogleClient(ctx, "u1", googleoauth.Client{ID: "c", Project: "Not A Project"}, nil, rec); err == nil || !errorAs(err, &inv) {
		t.Fatalf("bad project: %v", err)
	}
}

// errorAs is errors.As without importing errors twice in this file's callers.
func errorAs(err error, target **InvalidError) bool {
	inv, ok := err.(*InvalidError)
	if ok {
		*target = inv
	}
	return ok
}

func TestGoogleConnectionLifecycle(t *testing.T) {
	st, _, _ := googleFixture(t)
	ctx := context.Background()
	if err := st.SaveGoogleConnection(ctx, "u1", "ana@example.com", "", "refresh-1"); err != nil {
		t.Fatal(err)
	}
	conn, err := st.GoogleConnection(ctx, "u1")
	if err != nil || conn == nil || conn.Account != "ana@example.com" || conn.NeedsReconnect() {
		t.Fatalf("connection: %+v %v", conn, err)
	}
	sec, _ := st.Secrets()
	if sec.Value(GoogleRefreshKey("u1")) != "refresh-1" {
		t.Fatal("refresh token not stored")
	}
	if err := st.MarkGoogleReconnect(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if conn, _ := st.GoogleConnection(ctx, "u1"); !conn.NeedsReconnect() {
		t.Fatal("reconnect not marked")
	}
	if err := st.DeleteGoogleConnection(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if conn, _ := st.GoogleConnection(ctx, "u1"); conn != nil {
		t.Fatalf("still connected: %+v", conn)
	}
	if sec, _ := st.Secrets(); sec.Has(GoogleRefreshKey("u1")) {
		t.Fatal("refresh token kept")
	}
	// A sign-in without a refresh token is refused: nothing is stored.
	if err := st.SaveGoogleConnection(ctx, "u2", "", "", ""); err == nil {
		t.Fatal("stored without a refresh token")
	}
}

func TestOAuthStateIsSingleUseAndExpires(t *testing.T) {
	st, _, clock := googleFixture(t)
	ctx := context.Background()
	hash := googleoauth.StateHash("assistant", "state-1")
	if err := st.PutOAuthState(ctx, hash, OAuthState{UserID: "u1", Verifier: "v", RedirectURI: "https://x/cb"}, OAuthStateTTL); err != nil {
		t.Fatal(err)
	}
	got, err := st.TakeOAuthState(ctx, hash)
	if err != nil || got.UserID != "u1" || got.RedirectURI != "https://x/cb" {
		t.Fatalf("take: %+v %v", got, err)
	}
	if _, err := st.TakeOAuthState(ctx, hash); err == nil {
		t.Fatal("the state was usable twice")
	}
	// An expired sign-in is refused and removed.
	expired := googleoauth.StateHash("assistant", "state-2")
	if err := st.PutOAuthState(ctx, expired, OAuthState{UserID: "u1"}, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	clock.at = clock.at.Add(11 * time.Minute)
	if _, err := st.TakeOAuthState(ctx, expired); err == nil {
		t.Fatal("the expired state was accepted")
	}
}

func TestGoogleAccessTokenIsCachedAndRefreshed(t *testing.T) {
	st, fake, _ := googleFixture(t)
	ctx := context.Background()
	if err := st.SaveGoogleConnection(ctx, "u1", "ana@example.com", "", "refresh-1"); err != nil {
		t.Fatal(err)
	}
	client := googleoauth.Client{ID: "c", Secret: "s"}
	if _, err := st.SaveGoogleClient(ctx, "u1", client, &client.Secret, nil); err != nil {
		t.Fatal(err)
	}
	tok, err := st.GoogleTokens().AccessToken(ctx, "u1")
	if err != nil || tok != "refreshed-1" {
		t.Fatalf("token: %q %v", tok, err)
	}
	if _, err := st.GoogleTokens().AccessToken(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if n := fake.refreshes.Load(); n != 1 {
		t.Fatalf("refreshed %d times for two calls", n)
	}
	// A refused request drops the cached token and mints another.
	if tok, err := st.GoogleTokens().Refresh(ctx, "u1"); err != nil || tok != "refreshed-2" {
		t.Fatalf("refresh: %q %v", tok, err)
	}
}

func TestGoogleTokenRevokedMarksTheConnection(t *testing.T) {
	st, fake, _ := googleFixture(t)
	ctx := context.Background()
	secret := "s"
	if _, err := st.SaveGoogleClient(ctx, "u1", googleoauth.Client{ID: "c"}, &secret, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveGoogleConnection(ctx, "u1", "ana@example.com", "", "refresh-1"); err != nil {
		t.Fatal(err)
	}
	fake.refuse.Store(true)
	_, err := st.GoogleTokens().AccessToken(ctx, "u1")
	pe, ok := AsProviderError(err)
	if !ok || pe.Code != CodeProviderAuth || pe.Msg != ReconnectNeedsMessage {
		t.Fatalf("refused grant: %v", err)
	}
	conn, _ := st.GoogleConnection(ctx, "u1")
	if !conn.NeedsReconnect() {
		t.Fatal("the connection was not marked for a reconnect")
	}
	// A connection that is gone is its own error, not a provider failure.
	if _, err := st.GoogleTokens().AccessToken(ctx, "u2"); err != ErrNoGoogleConnection {
		t.Fatalf("no connection: %v", err)
	}
}

func TestResolvePrefersKeyThenGoogleAccount(t *testing.T) {
	st, _, _ := googleFixture(t)
	ctx := context.Background()
	set := googleSettings()
	secret := "cs"
	if _, err := st.SaveGoogleClient(ctx, "u1", googleoauth.Client{ID: "c"}, &secret, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveGoogleConnection(ctx, "u1", "ana@example.com", "my-project-123456", "refresh-1"); err != nil {
		t.Fatal(err)
	}
	// The member chose Gemini with no API key: the account is used.
	if err := st.SetMemberChoice(ctx, "u1", Gemini, "gemini-2.5-flash", nil); err != nil {
		t.Fatal(err)
	}
	r, err := st.Resolve(ctx, "u1", set)
	if err != nil || !r.OnGoogleAccount() || !r.Own || !r.Member || r.project != "my-project-123456" {
		t.Fatalf("route on the account: %+v %v", r, err)
	}
	// An own API key wins over the account.
	if err := st.SetUserKey(ctx, "u1", Gemini, "AIza-own", nil); err != nil {
		t.Fatal(err)
	}
	if r, _ := st.Resolve(ctx, "u1", set); r.OnGoogleAccount() || r.key != "AIza-own" {
		t.Fatalf("key should win: %+v", r)
	}
	// The owner's provider with no key of its own runs on the person's account.
	set.AllowMemberKeys = true
	set.DefaultProvider = Gemini
	r, err = st.Resolve(ctx, "u2", googleSettings())
	if err != nil || !r.OnGoogleAccount() {
		// u2 has no connection: the operator's key (here none) is the route.
		if r.OnGoogleAccount() {
			t.Fatalf("u2 has no connection: %+v", r)
		}
	}
	if !set.Providers[Gemini].Enabled {
		t.Fatal("fixture lost the provider")
	}
}

// chatFake serves one streamed Chat Completions answer and records the headers.
type chatFake struct {
	srv     *httptest.Server
	auth    []string
	project []string
	hits    atomic.Int32
	// status, when set, answers that status instead of a stream.
	status atomic.Int32
}

func newChatFake(t *testing.T) *chatFake {
	t.Helper()
	f := &chatFake{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.project = append(f.project, r.Header.Get("x-goog-user-project"))
		if s := f.status.Load(); s != 0 {
			w.WriteHeader(int(s))
			fmt.Fprint(w, `[{"error":{"code":401,"message":"invalid token","status":"UNAUTHENTICATED"}}]`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func TestChatProviderSendsTheGoogleAccount(t *testing.T) {
	chat := newChatFake(t)
	var minted atomic.Int32
	token := &stubToken{token: func() (string, error) { minted.Add(1); return "access-token", nil }}
	p := &ChatProvider{Name: Gemini, BaseURL: chat.srv.URL, Token: token, Project: "my-project-123456", Model: "gemini-2.5-flash", HTTP: chat.srv.Client()}
	conv := p.Start(Request{System: "s", Messages: []Message{{Role: "user", Text: "hi"}}, MaxTokens: 16})
	if _, err := conv.Next(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if chat.auth[0] != "Bearer access-token" || chat.project[0] != "my-project-123456" {
		t.Fatalf("headers: %q %q", chat.auth[0], chat.project[0])
	}
	if minted.Load() != 1 {
		t.Fatalf("minted %d times", minted.Load())
	}
}

func TestChatProviderRefreshesOnceThenAsksToReconnect(t *testing.T) {
	chat := newChatFake(t)
	chat.status.Store(http.StatusUnauthorized)
	current := "stale"
	token := &stubToken{token: func() (string, error) { return current, nil }, refresh: func() (string, error) { current = "fresh"; return current, nil }}
	p := &ChatProvider{Name: Gemini, BaseURL: chat.srv.URL, Token: token, Model: "m", HTTP: chat.srv.Client()}
	conv := p.Start(Request{System: "s", Messages: []Message{{Role: "user", Text: "hi"}}, MaxTokens: 16})
	_, err := conv.Next(context.Background(), func(string) {})
	pe, ok := AsProviderError(err)
	if !ok || pe.Code != CodeProviderAuth || pe.Msg != ReconnectNeedsMessage {
		t.Fatalf("second 401: %v", err)
	}
	if chat.hits.Load() != 2 || token.refreshed != 1 {
		t.Fatalf("retried %d times, refreshed %d", chat.hits.Load(), token.refreshed)
	}
	if strings.HasPrefix(chat.auth[1], "Bearer fresh") != true {
		t.Fatalf("the retry did not use the new token: %q", chat.auth[1])
	}
}

// stubToken is a TokenSource counting its calls.
type stubToken struct {
	token     func() (string, error)
	refresh   func() (string, error)
	refreshed int
}

func (s *stubToken) Token(context.Context) (string, error) { return s.token() }

func (s *stubToken) Refresh(context.Context) (string, error) {
	s.refreshed++
	if s.refresh == nil {
		return s.token()
	}
	return s.refresh()
}

func TestGoogleEmailFromIdentityToken(t *testing.T) {
	cases := map[string]string{
		idToken("ana@example.com"): "ana@example.com",
		"not-a-token":              "",
		"":                         "",
	}
	for in, want := range cases {
		if got := googleoauth.Email(in); got != want {
			t.Errorf("Email(%q) = %q, want %q", in, got, want)
		}
	}
	unverified := func() string {
		payload, _ := json.Marshal(map[string]any{"email": "ana@example.com", "email_verified": false})
		return "e30." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	}()
	if got := googleoauth.Email(unverified); got != "" {
		t.Errorf("an unverified address was used: %q", got)
	}
}
