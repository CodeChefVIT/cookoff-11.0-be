package utils

import (
	"crypto/subtle"
	"net/url"
)

// CallbackTokenParam is the query parameter carrying the shared secret on the
// Judge0 callback URL.
const CallbackTokenParam = "token"

// SignCallbackURL appends the callback secret to rawURL when one is
// configured. With no secret set the URL is returned untouched, so an existing
// deployment keeps working exactly as before.
func SignCallbackURL(rawURL string) string {
	secret := Config.Judge0CallbackSecret
	if secret == "" {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		// A malformed CALLBACK_URL is a deployment problem, not something to
		// paper over by silently dropping the secret — let Judge0 reject it.
		return rawURL
	}

	query := parsed.Query()
	query.Set(CallbackTokenParam, secret)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

// CallbackTokenValid reports whether a callback request carried the configured
// secret. Always true when no secret is configured.
func CallbackTokenValid(token string) bool {
	secret := Config.Judge0CallbackSecret
	if secret == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}
