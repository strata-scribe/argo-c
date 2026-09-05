package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGitLabReconciler_NotFound_CreatesWebhook(t *testing.T) {
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":123,"url":"https://example.com/hook"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewHTTPGitLabClient(server.URL, "test-token", server.Client())
	reconciler := NewGitLabReconciler(client, nil)

	desired := &GitLabWebhook{
		URL:        "https://example.com/hook",
		PushEvents: true,
	}

	hook, err := reconciler.Reconcile(context.Background(), "project", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 123 {
		t.Fatalf("expected hook ID 123, got: %+v", hook)
	}
	if atomic.LoadInt32(&createCalled) != 1 {
		t.Fatalf("expected CreateWebhook to be called once, got %d", createCalled)
	}
}

func TestGitLabReconciler_ServerError_HaltsWithoutCreation(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"InternalServerError", http.StatusInternalServerError},
		{"BadGateway", http.StatusBadGateway},
		{"ServiceUnavailable", http.StatusServiceUnavailable},
		{"GatewayTimeout", http.StatusGatewayTimeout},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var createCalled int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte(`{"message":"GitLab error"}`))
					return
				}
				if r.Method == http.MethodPost {
					atomic.AddInt32(&createCalled, 1)
					w.WriteHeader(http.StatusCreated)
					return
				}
			}))
			defer server.Close()

			client := NewHTTPGitLabClient(server.URL, "test-token", server.Client())
			reconciler := NewGitLabReconciler(client, nil)

			desired := &GitLabWebhook{
				URL: "https://example.com/hook",
			}

			_, err := reconciler.Reconcile(context.Background(), "project", desired)
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tc.statusCode)
			}
			if !errors.Is(err, ErrGitLabServerError) {
				t.Fatalf("expected ErrGitLabServerError, got: %v", err)
			}
			if atomic.LoadInt32(&createCalled) != 0 {
				t.Fatalf("CreateWebhook should NOT be called on server error %d", tc.statusCode)
			}
		})
	}
}

func TestGitLabReconciler_RateLimited_HaltsWithoutCreation(t *testing.T) {
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitLabClient(server.URL, "test-token", server.Client())
	reconciler := NewGitLabReconciler(client, nil)

	desired := &GitLabWebhook{
		URL: "https://example.com/hook",
	}

	_, err := reconciler.Reconcile(context.Background(), "project", desired)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}
	if !errors.Is(err, ErrGitLabRateLimited) {
		t.Fatalf("expected ErrGitLabRateLimited, got: %v", err)
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called on 429 Rate Limit")
	}
}

func TestGitLabReconciler_AuthErrors_HaltsWithoutCreation(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		expectedErr error
	}{
		{"Unauthorized", http.StatusUnauthorized, ErrGitLabUnauthorized},
		{"Forbidden", http.StatusForbidden, ErrGitLabForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var createCalled int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte(`{"message":"Auth error"}`))
					return
				}
				if r.Method == http.MethodPost {
					atomic.AddInt32(&createCalled, 1)
					w.WriteHeader(http.StatusCreated)
					return
				}
			}))
			defer server.Close()

			client := NewHTTPGitLabClient(server.URL, "invalid-token", server.Client())
			reconciler := NewGitLabReconciler(client, nil)

			desired := &GitLabWebhook{
				URL: "https://example.com/hook",
			}

			_, err := reconciler.Reconcile(context.Background(), "project", desired)
			if err == nil {
				t.Fatalf("expected auth error for %s, got nil", tc.name)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("expected %v, got: %v", tc.expectedErr, err)
			}
			if atomic.LoadInt32(&createCalled) != 0 {
				t.Fatalf("CreateWebhook should NOT be called on auth error %d", tc.statusCode)
			}
		})
	}
}

func TestGitLabReconciler_Timeout_HaltsWithoutCreation(t *testing.T) {
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitLabClient(server.URL, "test-token", server.Client())
	reconciler := NewGitLabReconciler(client, nil)

	desired := &GitLabWebhook{
		URL: "https://example.com/hook",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := reconciler.Reconcile(ctx, "project", desired)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called on timeout error")
	}
}

func TestGitLabReconciler_ExistingWebhook_Idempotent(t *testing.T) {
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":456,"url":"https://example.com/hook"}]`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitLabClient(server.URL, "test-token", server.Client())
	reconciler := NewGitLabReconciler(client, nil)

	desired := &GitLabWebhook{
		URL: "https://example.com/hook",
	}

	hook, err := reconciler.Reconcile(context.Background(), "project", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 456 {
		t.Fatalf("expected existing hook ID 456, got: %+v", hook)
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called when webhook already exists")
	}
}

func TestHTTPGitHubClient_UpdateWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/hooks/123" {
			t.Errorf("expected path /repos/owner/repo/hooks/123, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":123,"active":true,"events":["push","pull_request"],"config":{"url":"https://example.com/hook"}}`))
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())

	hook := &Webhook{
		ID:     123,
		Active: true,
		Events: []string{"push", "pull_request"},
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	updated, err := client.UpdateWebhook(context.Background(), "owner", "repo", hook)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.ID != 123 || !updated.Active || len(updated.Events) != 2 {
		t.Fatalf("unexpected updated webhook: %+v", updated)
	}
}

func TestHTTPGitHubClient_DeleteWebhook(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/hooks/123" {
			t.Errorf("expected path /repos/owner/repo/hooks/123, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())

	err := client.DeleteWebhook(context.Background(), "owner", "repo", 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReconciler_NotFound_CreatesWebhook(t *testing.T) {
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":123,"active":true,"config":{"url":"https://example.com/hook"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	reconciler.baseDelay = time.Millisecond

	desired := &Webhook{
		Active: true,
		Events: []string{"push"},
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	hook, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 123 {
		t.Fatalf("expected hook ID 123, got: %+v", hook)
	}
	if atomic.LoadInt32(&createCalled) != 1 {
		t.Fatalf("expected CreateWebhook to be called once, got %d", createCalled)
	}
}

func TestReconciler_ServerError_HaltsWithoutCreation(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"InternalServerError", http.StatusInternalServerError},
		{"BadGateway", http.StatusBadGateway},
		{"ServiceUnavailable", http.StatusServiceUnavailable},
		{"GatewayTimeout", http.StatusGatewayTimeout},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var createCalled int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte(`{"message":"GitHub error"}`))
					return
				}
				if r.Method == http.MethodPost {
					atomic.AddInt32(&createCalled, 1)
					w.WriteHeader(http.StatusCreated)
					return
				}
			}))
			defer server.Close()

			client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
			reconciler := NewReconciler(client, nil)
			reconciler.baseDelay = time.Millisecond

			desired := &Webhook{
				Active: true,
				Config: WebhookConfig{URL: "https://example.com/hook"},
			}

			_, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
			if err == nil {
				t.Fatalf("expected error for status %d, got nil", tc.statusCode)
			}
			if !errors.Is(err, ErrServerError) {
				t.Fatalf("expected ErrServerError, got: %v", err)
			}
			if atomic.LoadInt32(&createCalled) != 0 {
				t.Fatalf("CreateWebhook should NOT be called on server error %d", tc.statusCode)
			}
		})
	}
}

func TestReconciler_RateLimited_HaltsWithoutCreation(t *testing.T) {
	var createCalled int32
	var getCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&getCalled, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	reconciler.maxRetries = 0 // Disable retries for this test
	reconciler.baseDelay = time.Millisecond

	desired := &Webhook{
		Active: true,
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	_, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called on 429 Rate Limit")
	}
	if atomic.LoadInt32(&getCalled) != 1 {
		t.Fatalf("Expected exactly 1 get call, got %d", getCalled)
	}
}

func TestReconciler_RetriesOnRateLimit_ExceedsMaxRetries(t *testing.T) {
	var getCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			atomic.AddInt32(&getCalled, 1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	reconciler.maxRetries = 3
	reconciler.baseDelay = time.Millisecond

	desired := &Webhook{
		Active: true,
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	_, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got: %v", err)
	}

	// Should be 1 initial attempt + 3 retries = 4 attempts total
	expectedCalls := int32(4)
	if atomic.LoadInt32(&getCalled) != expectedCalls {
		t.Fatalf("expected %d get calls due to retries, got %d", expectedCalls, getCalled)
	}
}

func TestReconciler_RetriesOnRateLimit_SuccessAfterRetries(t *testing.T) {
	var getCalled int32
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			calls := atomic.AddInt32(&getCalled, 1)
			if calls <= 2 {
				// Fail the first two times with RateLimit
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
				return
			}
			// Succeed on the third attempt
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":456,"active":true,"config":{"url":"https://example.com/hook"}}]`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	reconciler.maxRetries = 3
	reconciler.baseDelay = time.Millisecond

	desired := &Webhook{
		Active: true,
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	hook, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 456 {
		t.Fatalf("expected existing hook ID 456, got: %+v", hook)
	}

	expectedGetCalls := int32(3) // 2 failures + 1 success
	if atomic.LoadInt32(&getCalled) != expectedGetCalls {
		t.Fatalf("expected %d get calls, got %d", expectedGetCalls, getCalled)
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called since webhook was found")
	}
}

func TestReconciler_AuthErrors_HaltsWithoutCreation(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		expectedErr error
	}{
		{"Unauthorized", http.StatusUnauthorized, ErrUnauthorized},
		{"Forbidden", http.StatusForbidden, ErrForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var createCalled int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(tc.statusCode)
					_, _ = w.Write([]byte(`{"message":"Auth error"}`))
					return
				}
				if r.Method == http.MethodPost {
					atomic.AddInt32(&createCalled, 1)
					w.WriteHeader(http.StatusCreated)
					return
				}
			}))
			defer server.Close()

			client := NewHTTPGitHubClient(server.URL, "invalid-token", server.Client())
			reconciler := NewReconciler(client, nil)
			reconciler.baseDelay = time.Millisecond

			desired := &Webhook{
				Active: true,
				Config: WebhookConfig{URL: "https://example.com/hook"},
			}

			_, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
			if err == nil {
				t.Fatalf("expected auth error for %s, got nil", tc.name)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("expected %v, got: %v", tc.expectedErr, err)
			}
			if atomic.LoadInt32(&createCalled) != 0 {
				t.Fatalf("CreateWebhook should NOT be called on auth error %d", tc.statusCode)
			}
		})
	}
}

func TestReconciler_Timeout_HaltsWithoutCreation(t *testing.T) {
	var createCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	reconciler.baseDelay = time.Millisecond

	desired := &Webhook{
		Active: true,
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := reconciler.Reconcile(ctx, "owner", "repo", desired)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called on timeout error")
	}
}

func TestReconciler_ExistingWebhook_Idempotent(t *testing.T) {
	var createCalled int32
	var updateCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":456,"active":true,"events":["push"],"config":{"url":"https://example.com/hook"}}]`))
			return
		}
		if r.Method == http.MethodPost {
			atomic.AddInt32(&createCalled, 1)
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method == http.MethodPatch {
			atomic.AddInt32(&updateCalled, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	reconciler.baseDelay = time.Millisecond

	desired := &Webhook{
		Active: true,
		Events: []string{"push"},
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	hook, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 456 {
		t.Fatalf("expected existing hook ID 456, got: %+v", hook)
	}
	if atomic.LoadInt32(&createCalled) != 0 {
		t.Fatalf("CreateWebhook should NOT be called when webhook already exists")
	}
	if atomic.LoadInt32(&updateCalled) != 0 {
		t.Fatalf("UpdateWebhook should NOT be called when webhook matches")
	}
}

func TestReconciler_ExistingWebhook_UpdatesActiveState(t *testing.T) {
	var updateCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":456,"active":false,"events":["push"],"config":{"url":"https://example.com/hook"}}]`))
			return
		}
		if r.Method == http.MethodPatch {
			atomic.AddInt32(&updateCalled, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":456,"active":true,"events":["push"],"config":{"url":"https://example.com/hook"}}`))
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)

	// Desired state is Active: true, but existing is Active: false
	desired := &Webhook{
		Active: true,
		Events: []string{"push"},
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	hook, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 456 || !hook.Active {
		t.Fatalf("expected updated hook ID 456 with active=true, got: %+v", hook)
	}
	if atomic.LoadInt32(&updateCalled) != 1 {
		t.Fatalf("UpdateWebhook should be called once when Active state differs")
	}
}

func TestReconciler_ExistingWebhook_UpdatesEvents(t *testing.T) {
	var updateCalled int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":456,"active":true,"events":["push"],"config":{"url":"https://example.com/hook"}}]`))
			return
		}
		if r.Method == http.MethodPatch {
			atomic.AddInt32(&updateCalled, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":456,"active":true,"events":["push","pull_request"],"config":{"url":"https://example.com/hook"}}`))
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)

	// Desired state has more events
	desired := &Webhook{
		Active: true,
		Events: []string{"push", "pull_request"},
		Config: WebhookConfig{URL: "https://example.com/hook"},
	}

	hook, err := reconciler.Reconcile(context.Background(), "owner", "repo", desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hook == nil || hook.ID != 456 || len(hook.Events) != 2 {
		t.Fatalf("expected updated hook ID 456 with 2 events, got: %+v", hook)
	}
	if atomic.LoadInt32(&updateCalled) != 1 {
		t.Fatalf("UpdateWebhook should be called once when Events differ")
	}
}

func TestDecodeAndRouteBitbucketCloud(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":123,"active":true,"config":{"url":"https://example.com/hook"}}`))
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	desired := &Webhook{Active: true, Config: WebhookConfig{URL: "https://example.com/hook"}}

	t.Run("ValidPayload", func(t *testing.T) {
		payload := []byte(`{"repository":{"name":"my-repo","workspace":{"slug":"my-workspace"}}}`)
		hook, err := reconciler.DecodeAndRouteBitbucketCloud(context.Background(), payload, desired)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hook == nil || hook.ID != 123 {
			t.Fatalf("expected hook ID 123, got: %+v", hook)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		payload := []byte(`{"repository": { "name": "my-repo" `)
		_, err := reconciler.DecodeAndRouteBitbucketCloud(context.Background(), payload, desired)
		if err == nil {
			t.Fatal("expected error for invalid json, got nil")
		}
	})

	t.Run("MissingFields", func(t *testing.T) {
		payload := []byte(`{"repository":{"name":"","workspace":{"slug":""}}}`)
		_, err := reconciler.DecodeAndRouteBitbucketCloud(context.Background(), payload, desired)
		if err == nil {
			t.Fatal("expected error for missing fields, got nil")
		}
		if err.Error() != "missing owner or repo in bitbucket cloud payload" {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}

func TestDecodeAndRouteBitbucketServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":123,"active":true,"config":{"url":"https://example.com/hook"}}`))
			return
		}
	}))
	defer server.Close()

	client := NewHTTPGitHubClient(server.URL, "test-token", server.Client())
	reconciler := NewReconciler(client, nil)
	desired := &Webhook{Active: true, Config: WebhookConfig{URL: "https://example.com/hook"}}

	t.Run("ValidPayloadWithSlug", func(t *testing.T) {
		payload := []byte(`{"repository":{"slug":"my-repo","project":{"key":"PROJ"}}}`)
		hook, err := reconciler.DecodeAndRouteBitbucketServer(context.Background(), payload, desired)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hook == nil || hook.ID != 123 {
			t.Fatalf("expected hook ID 123, got: %+v", hook)
		}
	})

	t.Run("ValidPayloadWithName", func(t *testing.T) {
		payload := []byte(`{"repository":{"name":"my-repo","project":{"key":"PROJ"}}}`)
		hook, err := reconciler.DecodeAndRouteBitbucketServer(context.Background(), payload, desired)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hook == nil || hook.ID != 123 {
			t.Fatalf("expected hook ID 123, got: %+v", hook)
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		payload := []byte(`{"repository": { "slug": "my-repo" `)
		_, err := reconciler.DecodeAndRouteBitbucketServer(context.Background(), payload, desired)
		if err == nil {
			t.Fatal("expected error for invalid json, got nil")
		}
	})

	t.Run("MissingFields", func(t *testing.T) {
		payload := []byte(`{"repository":{"slug":"","project":{"key":""}}}`)
		_, err := reconciler.DecodeAndRouteBitbucketServer(context.Background(), payload, desired)
		if err == nil {
			t.Fatal("expected error for missing fields, got nil")
		}
		if err.Error() != "missing owner or repo in bitbucket server payload" {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
}
