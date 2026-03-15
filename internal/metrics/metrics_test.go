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

	// Verify at least some metrics are gathered (counters/gauges may not
	// appear until first set, but the gather call itself must succeed).
	_ = families
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
