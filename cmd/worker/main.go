package main

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/temporalio/benchmark-workers/activities"
	"github.com/temporalio/benchmark-workers/workflows"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"
	"go.uber.org/automaxprocs/maxprocs"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// parseCommaSeparatedEnv parses a comma-separated environment variable and returns a slice
// If there's only one value but multiple namespaces are needed, it reuses that value
func parseCommaSeparatedEnv(envVar string, numNamespaces int) []string {
	value := os.Getenv(envVar)
	if value == "" {
		return make([]string, numNamespaces)
	}

	values := strings.Split(value, ",")
	for i, v := range values {
		values[i] = strings.TrimSpace(v)
	}

	// If we have fewer values than namespaces, repeat the last value
	if len(values) < numNamespaces {
		lastValue := values[len(values)-1]
		for len(values) < numNamespaces {
			values = append(values, lastValue)
		}
	}

	return values
}

func main() {
	if _, err := maxprocs.Set(); err != nil {
		log.Printf("WARNING: failed to set GOMAXPROCS: %v.\n", err)
	}

	namespaces := os.Getenv("TEMPORAL_NAMESPACE")
	if namespaces == "" {
		namespaces = "default"
	}

	// Parse comma-separated namespaces
	namespaceList := strings.Split(namespaces, ",")
	for i, ns := range namespaceList {
		namespaceList[i] = strings.TrimSpace(ns)
	}

	log.Printf("Creating workers for namespaces: %v", namespaceList)

	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	if taskQueue == "" {
		taskQueue = "benchmark"
	}

	// Parse comma-separated configuration values
	grpcEndpoints := parseCommaSeparatedEnv("TEMPORAL_GRPC_ENDPOINT", len(namespaceList))
	tlsKeyPaths := parseCommaSeparatedEnv("TEMPORAL_TLS_KEY", len(namespaceList))
	tlsCertPaths := parseCommaSeparatedEnv("TEMPORAL_TLS_CERT", len(namespaceList))
	tlsCaPaths := parseCommaSeparatedEnv("TEMPORAL_TLS_CA", len(namespaceList))

	// Create shared metrics handler if Prometheus is enabled
	var metricsHandler client.MetricsHandler
	if os.Getenv("PROMETHEUS_ENDPOINT") != "" {
		metricsHandler = sdktally.NewMetricsHandler(newPrometheusScope(prometheus.Configuration{
			ListenAddress: os.Getenv("PROMETHEUS_ENDPOINT"),
			TimerType:     "histogram",
		}))
	}

	// Create workers for each namespace
	var wg sync.WaitGroup
	workers := make([]worker.Worker, len(namespaceList))
	clients := make([]client.Client, len(namespaceList))

	for i, namespace := range namespaceList {
		clientOptions := client.Options{
			HostPort:  grpcEndpoints[i],
			Namespace: namespace,
		}

		tlsKeyPath := tlsKeyPaths[i]
		tlsCertPath := tlsCertPaths[i]
		tlsCaPath := tlsCaPaths[i]

		if tlsKeyPath != "" && tlsCertPath != "" {
			tlsConfig := tls.Config{}

			cert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
			if err != nil {
				log.Fatalf("Unable to create key pair for TLS for namespace %s: %v", namespace, err)
			}

			var tlsCaPool *x509.CertPool
			if tlsCaPath != "" {
				tlsCaPool = x509.NewCertPool()
				b, err := os.ReadFile(tlsCaPath)
				if err != nil {
					log.Fatalf("Failed reading server CA for namespace %s: %v", namespace, err)
				} else if !tlsCaPool.AppendCertsFromPEM(b) {
					log.Fatalf("Server CA PEM file invalid for namespace %s", namespace)
				}
			}

			tlsConfig.Certificates = []tls.Certificate{cert}
			tlsConfig.RootCAs = tlsCaPool

			if os.Getenv("TEMPORAL_TLS_DISABLE_HOST_VERIFICATION") != "" {
				tlsConfig.InsecureSkipVerify = true
			}

			clientOptions.ConnectionOptions.TLS = &tlsConfig
		}

		if metricsHandler != nil {
			clientOptions.MetricsHandler = metricsHandler
		}

		c, err := client.Dial(clientOptions)
		if err != nil {
			log.Fatalf("Unable to create client for namespace %s (endpoint: %s): %v", namespace, grpcEndpoints[i], err)
		}
		clients[i] = c

		workerOptions := worker.Options{}

		if os.Getenv("TEMPORAL_WORKFLOW_TASK_POLLERS") != "" {
			pollers, err := strconv.Atoi(os.Getenv("TEMPORAL_WORKFLOW_TASK_POLLERS"))
			if err != nil {
				log.Fatalf("TEMPORAL_WORKFLOW_TASK_POLLERS is invalid: %v", err)
			}
			workerOptions.MaxConcurrentWorkflowTaskPollers = pollers
		}

		if os.Getenv("TEMPORAL_ACTIVITY_TASK_POLLERS") != "" {
			pollers, err := strconv.Atoi(os.Getenv("TEMPORAL_ACTIVITY_TASK_POLLERS"))
			if err != nil {
				log.Fatalf("TEMPORAL_ACTIVITY_TASK_POLLERS is invalid: %v", err)
			}
			workerOptions.MaxConcurrentActivityTaskPollers = pollers
		}

		// TODO: Support more worker options

		w := worker.New(c, taskQueue, workerOptions)

		w.RegisterWorkflowWithOptions(workflows.ExecuteActivityWorkflow, workflow.RegisterOptions{Name: "ExecuteActivity"})
		w.RegisterWorkflowWithOptions(workflows.ReceiveSignalWorkflow, workflow.RegisterOptions{Name: "ReceiveSignal"})
		w.RegisterWorkflowWithOptions(workflows.DSLWorkflow, workflow.RegisterOptions{Name: "DSL"})
		w.RegisterActivityWithOptions(activities.SleepActivity, activity.RegisterOptions{Name: "Sleep"})
		w.RegisterActivityWithOptions(activities.EchoActivity, activity.RegisterOptions{Name: "Echo"})

		workers[i] = w
		log.Printf("Created worker for namespace: %s (endpoint: %s)", namespace, grpcEndpoints[i])
	}

	// Ensure all clients are closed on exit
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	// Start all workers concurrently
	for i, w := range workers {
		wg.Add(1)
		go func(w worker.Worker, namespace string) {
			defer wg.Done()
			log.Printf("Starting worker for namespace: %s", namespace)
			err := w.Run(worker.InterruptCh())
			if err != nil {
				log.Printf("Worker for namespace %s failed: %v", namespace, err)
			}
		}(w, namespaceList[i])
	}

	// Wait for all workers to complete
	wg.Wait()
}
