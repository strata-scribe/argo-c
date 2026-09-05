package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

var (
	// ErrInvalidSignatureFormat indicates the signature header doesn't match the expected format.
	ErrInvalidSignatureFormat = errors.New("invalid signature format")
	// ErrSignatureMismatch indicates the calculated signature doesn't match the provided one.
	ErrSignatureMismatch = errors.New("signature mismatch")
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

// ValidateSignature verifies the X-Hub-Signature-256 header using the provided payload and secret token.
// The signature must start with "sha256=" followed by the hex-encoded HMAC-SHA256 hash.
// It uses constant-time comparison to prevent timing attacks.
func ValidateSignature(signatureHeader string, payload []byte, secretToken string) error {
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return ErrInvalidSignatureFormat
	}

	signatureHex := signatureHeader[len(prefix):]
	expectedMAC, err := hex.DecodeString(signatureHex)
	if err != nil {
		return ErrInvalidSignatureFormat
	}

	mac := hmac.New(sha256.New, []byte(secretToken))
	mac.Write(payload)
	calculatedMAC := mac.Sum(nil)

	if !hmac.Equal(calculatedMAC, expectedMAC) {
		return ErrSignatureMismatch
	}

	return nil
}
