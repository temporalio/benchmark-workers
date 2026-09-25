package main

import (
	"errors"
	"testing"
)

func TestInvocationMetricsContract(t *testing.T) {
	m := newInvocationMetrics()
	failure := errors.New("invocation failed")

	if err := m.run(true, func() error { return nil }); err != nil {
		t.Fatalf("waited success: %v", err)
	}
	if err := m.run(true, func() error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("waited failure: got %v, want %v", err, failure)
	}
	if err := m.run(false, func() error { return nil }); err != nil {
		t.Fatalf("unwaited submission: %v", err)
	}
	if err := m.run(false, func() error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("unwaited submission failure: got %v, want %v", err, failure)
	}

	families, err := m.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	counters := map[string]float64{
		"benchmark_runner_invocations_started_total":   4,
		"benchmark_runner_invocations_completed_total": 1,
		"benchmark_runner_invocations_failed_total":    2,
	}
	histogramSeen := false
	for _, family := range families {
		if want, ok := counters[family.GetName()]; ok {
			if len(family.GetMetric()) != 1 || family.GetMetric()[0].GetCounter().GetValue() != want {
				t.Errorf("%s: got %v, want %v", family.GetName(), family.GetMetric(), want)
			}
			delete(counters, family.GetName())
		}
		if family.GetName() != "benchmark_runner_invocation_duration_seconds" {
			continue
		}
		histogramSeen = true
		counts := map[string]uint64{"completed": 1, "failed": 2}
		for _, metric := range family.GetMetric() {
			if len(metric.GetLabel()) != 1 || metric.GetLabel()[0].GetName() != "outcome" {
				t.Errorf("unexpected duration labels: %v", metric.GetLabel())
				continue
			}
			outcome := metric.GetLabel()[0].GetValue()
			want, ok := counts[outcome]
			if !ok {
				t.Errorf("unexpected outcome: %s", outcome)
				continue
			}
			histogram := metric.GetHistogram()
			if histogram.GetSampleCount() != want {
				t.Errorf("%s sample count: got %d, want %d", outcome, histogram.GetSampleCount(), want)
			}
			for _, bound := range []float64{0.005, 0.1, 1, 60, 600} {
				found := false
				for _, bucket := range histogram.GetBucket() {
					if bucket.GetUpperBound() == bound {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("%s histogram missing bucket %g", outcome, bound)
				}
			}
			delete(counts, outcome)
		}
		if len(counts) != 0 {
			t.Errorf("missing duration outcomes: %v", counts)
		}
	}
	if len(counters) != 0 {
		t.Errorf("missing counters: %v", counters)
	}
	if !histogramSeen {
		t.Error("missing invocation duration histogram")
	}
}
