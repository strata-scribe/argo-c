package webhook

import (
	"testing"
	"time"
)

func TestBackoffCalculator_ConstantBackoff(t *testing.T) {
	calc := &BackoffCalculator{
		Policy:     ConstantBackoff,
		BaseDelay:  time.Second,
		MaxRetries: 3,
	}

	for i := 0; i < 3; i++ {
		delay, ok := calc.Calculate(i)
		if !ok {
			t.Errorf("expected attempt %d to be allowed", i)
		}
		if delay != time.Second {
			t.Errorf("expected delay %v, got %v", time.Second, delay)
		}
	}
}

func TestBackoffCalculator_LinearBackoff(t *testing.T) {
	calc := &BackoffCalculator{
		Policy:     LinearBackoff,
		BaseDelay:  time.Second,
		MaxRetries: 3,
	}

	expectedDelays := []time.Duration{
		time.Second,
		2 * time.Second,
		3 * time.Second,
		4 * time.Second,
	}

	for i := 0; i <= 3; i++ {
		delay, ok := calc.Calculate(i)
		if !ok {
			t.Errorf("expected attempt %d to be allowed", i)
		}
		if delay != expectedDelays[i] {
			t.Errorf("expected delay %v for attempt %d, got %v", expectedDelays[i], i, delay)
		}
	}
}

func TestBackoffCalculator_ExponentialBackoff(t *testing.T) {
	calc := &BackoffCalculator{
		Policy:     ExponentialBackoff,
		BaseDelay:  time.Second,
		MaxRetries: 4,
	}

	expectedDelays := []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}

	for i := 0; i <= 4; i++ {
		delay, ok := calc.Calculate(i)
		if !ok {
			t.Errorf("expected attempt %d to be allowed", i)
		}
		if delay != expectedDelays[i] {
			t.Errorf("expected delay %v for attempt %d, got %v", expectedDelays[i], i, delay)
		}
	}
}

func TestBackoffCalculator_MaxRetries(t *testing.T) {
	calc := &BackoffCalculator{
		Policy:     ConstantBackoff,
		BaseDelay:  time.Second,
		MaxRetries: 2,
	}

	_, ok := calc.Calculate(0)
	if !ok {
		t.Errorf("expected attempt 0 to be allowed")
	}

	_, ok = calc.Calculate(1)
	if !ok {
		t.Errorf("expected attempt 1 to be allowed")
	}

	_, ok = calc.Calculate(2)
	if !ok {
		t.Errorf("expected attempt 2 to be allowed")
	}

	_, ok = calc.Calculate(3)
	if ok {
		t.Errorf("expected attempt 3 to be rejected")
	}
}

func TestBackoffCalculator_MaxDelay(t *testing.T) {
	calc := &BackoffCalculator{
		Policy:     ExponentialBackoff,
		BaseDelay:  time.Second,
		MaxDelay:   5 * time.Second,
		MaxRetries: 4,
	}

	expectedDelays := []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		5 * time.Second,
		5 * time.Second,
	}

	for i := 0; i <= 4; i++ {
		delay, ok := calc.Calculate(i)
		if !ok {
			t.Errorf("expected attempt %d to be allowed", i)
		}
		if delay != expectedDelays[i] {
			t.Errorf("expected delay %v for attempt %d, got %v", expectedDelays[i], i, delay)
		}
	}
}

func TestBackoffCalculator_Jitter(t *testing.T) {
	baseDelay := 10 * time.Second
	calc := &BackoffCalculator{
		Policy:     ConstantBackoff,
		BaseDelay:  baseDelay,
		MaxRetries: 100,
		Jitter:     true,
	}

	// Because jitter is +/- 20%
	minDelay := time.Duration(float64(baseDelay) * 0.8)
	maxDelay := time.Duration(float64(baseDelay) * 1.2)

	for i := 0; i < 100; i++ {
		delay, ok := calc.Calculate(i)
		if !ok {
			t.Fatalf("expected attempt %d to be allowed", i)
		}

		if delay < minDelay || delay > maxDelay {
			t.Errorf("expected delay to be between %v and %v, got %v", minDelay, maxDelay, delay)
		}
	}
}
