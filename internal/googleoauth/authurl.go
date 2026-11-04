package googleoauth

import (
	"net/url"
	"strings"
)

// AuthorizationURL is Google's consent page for one sign-in: offline access
// (a refresh token), a forced consent so that reconnecting always returns
// one, the S256 challenge, and the state that ties the answer to the row the
// caller stored. An empty authURL means Google's own endpoint.
func AuthorizationURL(authURL string, r AuthRequest) string {
	if authURL == "" {
		authURL = AuthURL
	}
	q := url.Values{
		"client_id":              {r.ClientID},
		"redirect_uri":           {r.RedirectURI},
		"response_type":          {"code"},
		"scope":                  {strings.Join(r.Scopes, " ")},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"include_granted_scopes": {"true"},
		"state":                  {r.State},
		"code_challenge":         {r.Challenge},
		"code_challenge_method":  {"S256"},
	}
	if r.LoginHint != "" {
		q.Set("login_hint", r.LoginHint)
	}
	return authURL + "?" + q.Encode()
}
