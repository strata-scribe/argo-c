package server

import (
	"encoding/json"
	"net/http"
)

// Probes encapsulates the dependencies for health checks.
type Probes struct {
	CheckGitConnectivity func() error
	CheckWebhookListener func() error
}

// ReadyzResponse represents the JSON response body for /readyz
type ReadyzResponse struct {
	GitConnectivity string `json:"git_connectivity"`
	WebhookListener string `json:"webhook_listener"`
}

// Healthz is a liveness probe handler. It returns 200 OK to indicate the server is running.
func (p *Probes) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Readyz is a readiness probe handler. It checks git connectivity and webhook listener health.
func (p *Probes) Readyz(w http.ResponseWriter, r *http.Request) {
	resp := ReadyzResponse{
		GitConnectivity: "ok",
		WebhookListener: "ok",
	}

	statusCode := http.StatusOK

	if p.CheckGitConnectivity != nil {
		if err := p.CheckGitConnectivity(); err != nil {
			resp.GitConnectivity = err.Error()
			statusCode = http.StatusServiceUnavailable
		}
	} else {
		resp.GitConnectivity = "not configured"
	}

	if p.CheckWebhookListener != nil {
		if err := p.CheckWebhookListener(); err != nil {
			resp.WebhookListener = err.Error()
			statusCode = http.StatusServiceUnavailable
		}
	} else {
		resp.WebhookListener = "not configured"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
