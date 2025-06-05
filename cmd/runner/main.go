package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/alitto/pond"
	"github.com/pborman/uuid"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"
	"go.uber.org/automaxprocs/maxprocs"

	"go.temporal.io/sdk/client"
)

var nWorfklows = flag.Int("c", 10, "concurrent workflows")
var sWorkflow = flag.String("t", "", "workflow type")
var sSignalType = flag.String("s", "", "signal type")
var bWait = flag.Bool("w", true, "wait for workflows to complete")
var sNamespace = flag.String("n", "default", "namespace (comma-separated list supported)")
var sTaskQueue = flag.String("tq", "benchmark", "task queue")

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
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [workflow input] ...\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if _, err := maxprocs.Set(); err != nil {
		log.Printf("WARNING: failed to set GOMAXPROCS: %v.\n", err)
	}

	namespaces := *sNamespace
	envNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if envNamespace != "" && envNamespace != "default" {
		namespaces = envNamespace
	}

	// Parse comma-separated namespaces
	namespaceList := strings.Split(namespaces, ",")
	for i, ns := range namespaceList {
		namespaceList[i] = strings.TrimSpace(ns)
	}

	log.Printf("Using namespaces: %v", namespaceList)

	// Parse comma-separated configuration values
	grpcEndpoints := parseCommaSeparatedEnv("TEMPORAL_GRPC_ENDPOINT", len(namespaceList))
	tlsKeyPaths := parseCommaSeparatedEnv("TEMPORAL_TLS_KEY", len(namespaceList))
	tlsCertPaths := parseCommaSeparatedEnv("TEMPORAL_TLS_CERT", len(namespaceList))
	tlsCaPaths := parseCommaSeparatedEnv("TEMPORAL_TLS_CA", len(namespaceList))

	// Create clients for each namespace
	clients := make([]client.Client, len(namespaceList))
	for i, namespace := range namespaceList {
		clientOptions := client.Options{
			HostPort:  grpcEndpoints[i],
			Namespace: namespace,
			Logger:    NewNopLogger(),
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

		if os.Getenv("PROMETHEUS_ENDPOINT") != "" {
			clientOptions.MetricsHandler = sdktally.NewMetricsHandler(newPrometheusScope(prometheus.Configuration{
				ListenAddress: os.Getenv("PROMETHEUS_ENDPOINT"),
				TimerType:     "histogram",
			}))
		}

		c, err := client.Dial(clientOptions)
		if err != nil {
			log.Fatalf("Unable to create client for namespace %s (endpoint: %s): %v", namespace, grpcEndpoints[i], err)
		}
		clients[i] = c
		log.Printf("Created client for namespace: %s (endpoint: %s)", namespace, grpcEndpoints[i])
	}

	// Ensure all clients are closed on exit
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	var input []interface{}
	for _, a := range flag.Args() {
		var i interface{}
		err := json.Unmarshal([]byte(a), &i)
		if err != nil {
			log.Fatalln("Unable to parse input", err)
		}
		input = append(input, i)
	}

	pool := pond.New(*nWorfklows, 0)

	// Counter for rotating among clients
	var clientCounter uint64

	var starter func() (client.WorkflowRun, error)

	if *sSignalType != "" {
		starter = func() (client.WorkflowRun, error) {
			// Rotate among clients
			clientIndex := atomic.AddUint64(&clientCounter, 1) % uint64(len(clients))
			c := clients[clientIndex]

			wID := uuid.New()
			return c.SignalWithStartWorkflow(
				context.Background(),
				wID,
				*sSignalType,
				nil,
				client.StartWorkflowOptions{
					ID:        wID,
					TaskQueue: *sTaskQueue,
				},
				*sWorkflow,
				input...,
			)
		}
	} else {
		starter = func() (client.WorkflowRun, error) {
			// Rotate among clients
			clientIndex := atomic.AddUint64(&clientCounter, 1) % uint64(len(clients))
			c := clients[clientIndex]

			return c.ExecuteWorkflow(
				context.Background(),
				client.StartWorkflowOptions{
					TaskQueue: *sTaskQueue,
				},
				*sWorkflow,
				input...,
			)
		}
	}

	go (func() {
		for {
			pool.Submit(func() {
				wf, err := starter()
				if err != nil {
					log.Println("Unable to start workflow", err)
					return
				}

				if *bWait {
					err = wf.Get(context.Background(), nil)
					if err != nil {
						log.Println("Workflow failed", err)
						return
					}
				}
			})
		}
	})()

	var lastCompleted uint64
	lastCheck := time.Now()

	for {
		rate := float64(pool.CompletedTasks()-lastCompleted) / time.Since(lastCheck).Seconds()

		fmt.Printf("Concurrent: %d Workflows: %d Rate: %f\n", pool.RunningWorkers(), pool.CompletedTasks(), rate)

		lastCheck = time.Now()
		lastCompleted = pool.CompletedTasks()

		time.Sleep(10 * time.Second)
	}
}
