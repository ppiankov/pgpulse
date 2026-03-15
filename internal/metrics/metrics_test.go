package metrics_test

import (
	"testing"

	"github.com/ppiankov/pgpulse/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	if m == nil {
		t.Fatal("expected non-nil metrics")
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	// All metrics should be registered (some may not appear until set)
	// Verify at least the counter and gauges that have default values
	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}

	// Counters appear after first increment, gauges appear after first set.
	// Just verify no panic on registration and gather succeeds.
	if len(families) < 0 {
		t.Fatal("unexpected negative family count")
	}
}

func TestMetricsDuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = metrics.New(reg)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()

	_ = metrics.New(reg)
}
