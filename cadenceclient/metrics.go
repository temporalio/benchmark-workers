package cadenceclient

import (
	"fmt"
	"log"
	"time"

	prom "github.com/m3db/prometheus_client_golang/prometheus"
	"github.com/uber-go/tally"
	"github.com/uber-go/tally/prometheus"
)

// prometheusSanitizeOptions keeps metric and label names Prometheus-compatible
// (alphanumeric plus underscore). The Temporal build relied on the SDK's
// contrib/tally sanitiser for this; the Cadence client takes a plain
// tally.Scope, so we apply the equivalent sanitisation ourselves.
var prometheusSanitizeOptions = tally.SanitizeOptions{
	NameCharacters:       tally.ValidCharacters{Ranges: tally.AlphanumericRange, Characters: tally.UnderscoreCharacters},
	KeyCharacters:        tally.ValidCharacters{Ranges: tally.AlphanumericRange, Characters: tally.UnderscoreCharacters},
	ValueCharacters:      tally.ValidCharacters{Ranges: tally.AlphanumericRange, Characters: tally.UnderscoreCharacters},
	ReplacementCharacter: tally.DefaultReplacementCharacter,
}

// newPrometheusScope creates a tally scope backed by a Prometheus reporter that
// serves metrics on listenAddress.
func newPrometheusScope(listenAddress string) (tally.Scope, error) {
	reporter, err := prometheus.Configuration{
		ListenAddress: listenAddress,
		TimerType:     "histogram",
	}.NewReporter(
		prometheus.ConfigurationOptions{
			Registry: prom.NewRegistry(),
			OnError: func(err error) {
				log.Println("error in prometheus reporter", err)
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error creating prometheus reporter: %w", err)
	}

	scopeOpts := tally.ScopeOptions{
		CachedReporter:  reporter,
		Separator:       prometheus.DefaultSeparator,
		SanitizeOptions: &prometheusSanitizeOptions,
	}
	scope, _ := tally.NewRootScope(scopeOpts, time.Second)

	log.Println("prometheus metrics scope created")
	return scope, nil
}
