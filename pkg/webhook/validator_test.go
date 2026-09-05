package webhook

import (
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
			name:        "empty secret but has header",
			headerToken: "some-token",
			secret:      "",
			expected:    false,
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
			// When headerToken is empty but we explicitly want it to be empty string for testing empty cases
			// http.Header.Get returns empty string if not set, which matches the empty headerToken behavior.

			if got := ValidateGitLabToken(req, tt.secret); got != tt.expected {
				t.Errorf("ValidateGitLabToken() = %v, want %v", got, tt.expected)
			}
		})
	}
}
