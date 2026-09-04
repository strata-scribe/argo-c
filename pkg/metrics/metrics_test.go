package metrics

import (
	"testing"
)

func TestMetricsRegistered(t *testing.T) {
	// The init function runs when this package is imported/tested,
	// registering the metrics. If the registration panics, the test will fail.

	// We can ensure the metrics are initialized and not nil.
	if WebhooksReceivedTotal == nil {
		t.Errorf("expected WebhooksReceivedTotal to be initialized")
	}
	if ReconciliationDurationSeconds == nil {
		t.Errorf("expected ReconciliationDurationSeconds to be initialized")
	}
	if ReconciliationErrorsTotal == nil {
		t.Errorf("expected ReconciliationErrorsTotal to be initialized")
	}
}
