package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateGitLabToken(t *testing.T) {
	tests := []struct {
		name        string
		headerToken string
		secret      string
		expected    bool
	}{
		{
			name:        "matching tokens",
			headerToken: "super-secret-token",
			secret:      "super-secret-token",
			expected:    true,
		},
		{
			name:        "non-matching tokens",
			headerToken: "wrong-token",
			secret:      "super-secret-token",
			expected:    false,
		},
		{
			name:        "missing header",
			headerToken: "",
			secret:      "super-secret-token",
			expected:    false,
		},
		{
			name:        "empty secret and empty header",
			headerToken: "",
			secret:      "",
			expected:    true,
		},
		{
			name:        "different lengths",
			headerToken: "short",
			secret:      "longer-secret",
			expected:    false,
		},
		{
			name:        "same length different content",
			headerToken: "secret123",
			secret:      "secret456",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.headerToken != "" {
				req.Header.Set("X-Gitlab-Token", tt.headerToken)
			}

			if got := ValidateGitLabToken(req, tt.secret); got != tt.expected {
				t.Errorf("ValidateGitLabToken() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func generateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestValidateSignature(t *testing.T) {
	secret := "my-secret-token"
	payload := []byte(`{"action":"opened","issue":{"number":1}}`)
	validSignature := generateSignature(payload, secret)

	tests := []struct {
		name      string
		signature string
		payload   []byte
		secret    string
		wantErr   error
	}{
		{
			name:      "Valid signature",
			signature: validSignature,
			payload:   payload,
			secret:    secret,
			wantErr:   nil,
		},
		{
			name:      "Invalid signature format (missing prefix)",
			signature: hex.EncodeToString(hmac.New(sha256.New, []byte(secret)).Sum(nil)),
			payload:   payload,
			secret:    secret,
			wantErr:   ErrInvalidSignatureFormat,
		},
		{
			name:      "Invalid signature format (invalid hex)",
			signature: "sha256=invalidhex",
			payload:   payload,
			secret:    secret,
			wantErr:   ErrInvalidSignatureFormat,
		},
		{
			name:      "Incorrect signature (mismatched secret)",
			signature: validSignature,
			payload:   payload,
			secret:    "wrong-secret",
			wantErr:   ErrSignatureMismatch,
		},
		{
			name:      "Mismatched payload",
			signature: validSignature,
			payload:   []byte(`{"action":"closed","issue":{"number":1}}`),
			secret:    secret,
			wantErr:   ErrSignatureMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSignature(tt.signature, tt.payload, tt.secret)
			if err != tt.wantErr {
				t.Errorf("ValidateSignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
