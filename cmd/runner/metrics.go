package main

import (
	"log"
	"time"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"
)

// Dense buckets below a minute make p90/p99 useful for short invocations,
// while the tail keeps slower workloads visible instead of pinning them at
// 60 seconds.
var invocationDurationBuckets = []float64{
	0.005, 0.0075, 0.01, 0.015, 0.02, 0.03, 0.04, 0.05,
	0.075, 0.1, 0.15, 0.2, 0.25, 0.3, 0.4, 0.5, 0.75,
	1, 1.25, 1.5, 2, 2.5, 3, 4, 5, 6, 7.5, 10,
	12.5, 15, 20, 25, 30, 40, 45, 50, 60,
	90, 120, 180, 300, 600,
}

type invocationMetrics struct {
	registry  *prom.Registry
	started   prom.Counter
	completed prom.Counter
	failed    prom.Counter
	duration  *prom.HistogramVec
}

func newInvocationMetrics() *invocationMetrics {
	m := &invocationMetrics{
		registry: prom.NewRegistry(),
		started: prom.NewCounter(prom.CounterOpts{
			Name: "benchmark_runner_invocations_started_total",
			Help: "Number of top-level invocation attempts made by the runner.",
		}),
		completed: prom.NewCounter(prom.CounterOpts{
			Name: "benchmark_runner_invocations_completed_total",
			Help: "Number of top-level invocations the runner waited for and observed complete successfully.",
		}),
		failed: prom.NewCounter(prom.CounterOpts{
			Name: "benchmark_runner_invocations_failed_total",
			Help: "Number of top-level invocations that failed to start or failed while waiting for completion.",
		}),
		duration: prom.NewHistogramVec(prom.HistogramOpts{
			Name:    "benchmark_runner_invocation_duration_seconds",
			Help:    "Client-observed wall time from before submission to completion or failure.",
			Buckets: invocationDurationBuckets,
		}, []string{"outcome"}),
	}
	m.registry.MustRegister(m.started, m.completed, m.failed, m.duration)
	return m
}

// run measures one top-level invocation. A successful non-waiting submission
// has no known completion time, so it is neither a completion nor a duration.
func (m *invocationMetrics) run(wait bool, invoke func() error) error {
	m.started.Inc()
	start := time.Now()
	err := invoke()
	if err != nil {
		m.failed.Inc()
		m.duration.WithLabelValues("failed").Observe(time.Since(start).Seconds())
	} else if wait {
		m.completed.Inc()
		m.duration.WithLabelValues("completed").Observe(time.Since(start).Seconds())
	}
	return err
}

func newPrometheusScope(c prometheus.Configuration, registry *prom.Registry) tally.Scope {
	reporter, err := c.NewReporter(
		prometheus.ConfigurationOptions{
			Registry: registry,
			OnError: func(err error) {
				log.Println("error in prometheus reporter", err)
			},
		},
	)
	if err != nil {
		log.Fatalln("error creating prometheus reporter", err)
	}
	scopeOpts := tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &sdktally.PrometheusSanitizeOptions,
	}
	scope, _ := tally.NewRootScope(scopeOpts, time.Second)

	log.Println("prometheus metrics scope created")
	return scope
}
