// Package googleoauth is Google's OAuth 2.0 authorization-code flow with
// PKCE, shared by the features that hold a person's Google account: the in-app
// assistant's Gemini connection first (assistant-gemini-oauth).
//
// It is a leaf package on the standard library: a client (the owner's client
// id and secret), a pending sign-in's state and PKCE verifier, the consent
// address, the token endpoint and the short-lived access tokens Google hands
// out. Secrets are never stored here; the caller keeps the refresh token in
// its own secrets file.
package googleoauth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/OpenTodoApp/opentodo/internal/ids"
)

// Google's endpoints.
const (
	AuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenURL = "https://oauth2.googleapis.com/token"
)

// Client is an OAuth client of the server owner: the id and secret of a
// "Web application" or "Desktop" client in a Google Cloud project, and the
// project whose quota the calls are attributed to (optional).
type Client struct {
	ID      string
	Secret  string
	Project string
}

// Complete reports whether both values are there.
func (c Client) Complete() bool { return c.ID != "" && c.Secret != "" }

// AuthRequest is everything the consent page needs.
type AuthRequest struct {
	ClientID    string
	RedirectURI string
	State       string
	Challenge   string
	Scopes      []string
	// LoginHint prefills Google's account chooser (the connected address).
	LoginHint string
}

// PKCE returns a new code verifier and its S256 challenge.
func PKCE() (verifier, challenge string) {
	verifier = ids.Secret(48)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// NewState returns a fresh state for one sign-in.
func NewState() string { return ids.Secret(32) }

// StateHash is how a state is stored: namespaced so two features sharing a
// table never collide, and one-way so the stored row cannot be replayed from
// a database dump.
func StateHash(namespace, state string) string {
	sum := sha256.Sum256([]byte("googleoauth\x00" + namespace + "\x00" + state))
	return hex.EncodeToString(sum[:])
}
