package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	p := &Probes{}

	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(p.Healthz)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "ok"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name                 string
		checkGit             func() error
		checkWebhook         func() error
		expectedStatus       int
		expectedGitStatus    string
		expectedWebhookStatus string
	}{
		{
			name:                  "all healthy",
			checkGit:              func() error { return nil },
			checkWebhook:          func() error { return nil },
			expectedStatus:        http.StatusOK,
			expectedGitStatus:     "ok",
			expectedWebhookStatus: "ok",
		},
		{
			name:                  "git connectivity failing",
			checkGit:              func() error { return errors.New("git unreachable") },
			checkWebhook:          func() error { return nil },
			expectedStatus:        http.StatusServiceUnavailable,
			expectedGitStatus:     "git unreachable",
			expectedWebhookStatus: "ok",
		},
		{
			name:                  "webhook listener failing",
			checkGit:              func() error { return nil },
			checkWebhook:          func() error { return errors.New("webhook offline") },
			expectedStatus:        http.StatusServiceUnavailable,
			expectedGitStatus:     "ok",
			expectedWebhookStatus: "webhook offline",
		},
		{
			name:                  "both failing",
			checkGit:              func() error { return errors.New("git unreachable") },
			checkWebhook:          func() error { return errors.New("webhook offline") },
			expectedStatus:        http.StatusServiceUnavailable,
			expectedGitStatus:     "git unreachable",
			expectedWebhookStatus: "webhook offline",
		},
		{
			name:                  "nil functions",
			checkGit:              nil,
			checkWebhook:          nil,
			expectedStatus:        http.StatusOK,
			expectedGitStatus:     "not configured",
			expectedWebhookStatus: "not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Probes{
				CheckGitConnectivity: tt.checkGit,
				CheckWebhookListener: tt.checkWebhook,
			}

			req, err := http.NewRequest("GET", "/readyz", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(p.Readyz)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.expectedStatus)
			}

			var resp ReadyzResponse
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}

			if resp.GitConnectivity != tt.expectedGitStatus {
				t.Errorf("handler returned unexpected git connectivity status: got %v want %v",
					resp.GitConnectivity, tt.expectedGitStatus)
			}

			if resp.WebhookListener != tt.expectedWebhookStatus {
				t.Errorf("handler returned unexpected webhook listener status: got %v want %v",
					resp.WebhookListener, tt.expectedWebhookStatus)
			}
		})
	}
}
