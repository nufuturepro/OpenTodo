package googleoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Error codes of a refused exchange or refresh.
const (
	// CodeUnauthorized is a grant or client Google refuses: the connection
	// must be made again (invalid_grant, unauthorized_client, invalid_client).
	CodeUnauthorized = "unauthorized"
	// CodeInvalid is any other refusal Google names in its answer.
	CodeInvalid = "invalid"
	// CodeRateLimited is Google asking to slow down.
	CodeRateLimited = "rate_limited"
	// CodeUnreachable is a transport failure or an answer this package cannot read.
	CodeUnreachable = "unreachable"
)

// Error is a failed token request. It carries no token and no code verifier.
type Error struct {
	Code   string
	Status int
	// Reason is Google's own error value (invalid_grant and so on), clipped.
	Reason string
	// Until is when a rate-limited request may be retried.
	Until time.Time
}

func (e *Error) Error() string {
	s := "googleoauth: " + e.Code
	if e.Status != 0 {
		s += " (HTTP " + itoa(e.Status) + ")"
	}
	if e.Reason != "" {
		s += ": " + e.Reason
	}
	return s
}

// Token is the outcome of an exchange or a refresh.
type Token struct {
	// Access is the bearer token sent to Google's APIs.
	Access string
	// Refresh is the new refresh token; empty when Google kept the old one.
	Refresh string
	// IDToken is Google's signed identity token (the openid scope); only an
	// exchange returns one.
	IDToken string
	// Expires is when Access stops working.
	Expires time.Time
}

// Endpoint talks to Google's token endpoint.
type Endpoint struct {
	HTTP *http.Client
	URL  string
	// Now is the clock (tests pin it); nil means time.Now.
	Now func() time.Time
}

// Exchange trades an authorization code for tokens.
func (e *Endpoint) Exchange(ctx context.Context, c Client, code, verifier, redirectURI string) (*Token, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
		"client_id":     {c.ID},
		"client_secret": {c.Secret},
	})
}

// Refresh gets a new access token; a revoked grant is CodeUnauthorized, which
// the caller reports as a connection to make again.
func (e *Endpoint) Refresh(ctx context.Context, c Client, refresh string) (*Token, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refresh},
		"client_id":     {c.ID},
		"client_secret": {c.Secret},
	})
}

// tokenAnswer is Google's answer to both grants.
type tokenAnswer struct {
	Access      string `json:"access_token"`
	Refresh     string `json:"refresh_token"`
	IDToken     string `json:"id_token"`
	ExpiresIn   int64  `json:"expires_in"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// maxBody bounds Google's answer.
const maxBody = 1 << 16

func (e *Endpoint) post(ctx context.Context, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &Error{Code: CodeInvalid}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := e.client().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &Error{Code: CodeUnreachable}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, &Error{Code: CodeUnreachable}
	}
	var out tokenAnswer
	_ = json.Unmarshal(body, &out)
	// Google asking to slow down, or failing on its side, is worth retrying
	// whatever its body names.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &Error{Code: CodeRateLimited, Status: resp.StatusCode, Until: retryAfter(resp.Header, e.now())}
	}
	if out.Error != "" {
		return nil, classify(resp.StatusCode, out.Error)
	}
	if resp.StatusCode != http.StatusOK || out.Access == "" {
		return nil, &Error{Code: CodeInvalid, Status: resp.StatusCode}
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 3600
	}
	return &Token{Access: out.Access, Refresh: out.Refresh, IDToken: out.IDToken,
		Expires: e.now().Add(time.Duration(out.ExpiresIn) * time.Second)}, nil
}

func (e *Endpoint) endpoint() string {
	if e.URL != "" {
		return e.URL
	}
	return TokenURL
}

func (e *Endpoint) client() *http.Client {
	if e.HTTP != nil {
		return e.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (e *Endpoint) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// classify maps Google's error value to a code. A refused grant or client is
// CodeUnauthorized: only connecting again helps.
func classify(status int, reason string) *Error {
	switch reason {
	case "invalid_grant", "unauthorized_client", "invalid_client", "consent_required", "access_denied":
		return &Error{Code: CodeUnauthorized, Status: status, Reason: clip(reason)}
	}
	return &Error{Code: CodeInvalid, Status: status, Reason: clip(reason)}
}

// retryAfter reads a Retry-After header in seconds (0 when absent).
func retryAfter(h http.Header, now time.Time) time.Time {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return time.Time{}
	}
	var secs int64
	for _, r := range v {
		if r < '0' || r > '9' {
			return time.Time{}
		}
		secs = secs*10 + int64(r-'0')
		if secs > 86_400 {
			secs = 86_400
			break
		}
	}
	return now.Add(time.Duration(secs) * time.Second)
}

// clip keeps Google's error value short and on one line.
func clip(s string) string {
	if len(s) > 64 {
		s = s[:64]
	}
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Cache keeps access tokens in memory (never on disk) for the caller's
// connections, keyed however the caller likes (a user id, a connection id).
type Cache struct {
	mu  sync.Mutex
	m   map[string]Token
	Now func() time.Time
}

// expirySlack is how long before its expiry a cached token is already stale,
// so a request never leaves with a token that dies in flight.
const expirySlack = time.Minute

// Get returns the cached access token of key when it is still usable.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.m[key]
	if !ok || t.Access == "" || c.now().Add(expirySlack).After(t.Expires) {
		return "", false
	}
	return t.Access, true
}

// Put stores the token of key.
func (c *Cache) Put(key string, t Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]Token{}
	}
	c.m[key] = t
}

// Drop forgets the token of key (after a refused request).
func (c *Cache) Drop(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

func (c *Cache) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
