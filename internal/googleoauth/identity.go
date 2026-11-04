package googleoauth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// Email is the address claim of the identity token of an exchange, or "".
//
// The token arrived over TLS from Google's own token endpoint in the request
// that is being answered, so its claim is trusted for what it is used for
// here: showing which account was connected. It is never used to authorize
// anything, and the signature is not checked.
func Email(idToken string) string {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	if !claims.EmailVerified || len(claims.Email) > 200 {
		return ""
	}
	return claims.Email
}
