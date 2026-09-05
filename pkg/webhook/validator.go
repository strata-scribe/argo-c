package webhook

import (
	"crypto/subtle"
	"net/http"
)

// ValidateGitLabToken validates the X-Gitlab-Token header in the request
// against the expected secret using constant-time comparison to prevent
// timing attacks.
func ValidateGitLabToken(req *http.Request, secret string) bool {
	headerToken := req.Header.Get("X-Gitlab-Token")

	// subtle.ConstantTimeCompare requires both slices to be the same length
	// to avoid leaking length information through early returns.
	if len(headerToken) != len(secret) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(headerToken), []byte(secret)) == 1
}
