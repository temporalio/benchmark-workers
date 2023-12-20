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
	"time"

	"github.com/alitto/pond"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"
	"go.uber.org/automaxprocs/maxprocs"

	"go.temporal.io/sdk/client"
)

var nWorfklows = flag.Int("c", 10, "concurrent workflows")
var sWorkflow = flag.String("t", "", "workflow type")
var bWait = flag.Bool("w", true, "wait for workflows to complete")
var sNamespace = flag.String("n", "default", "namespace")
var sTaskQueue = flag.String("tq", "benchmark", "task queue")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [workflow input] ...\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if _, err := maxprocs.Set(); err != nil {
		log.Printf("WARNING: failed to set GOMAXPROCS: %v.\n", err)
	}

	namespace := *sNamespace
	envNamespace := os.Getenv("TEMPORAL_NAMESPACE")
	if envNamespace != "" && envNamespace != "default" {
		namespace = envNamespace
	}

	clientOptions := client.Options{
		HostPort:  os.Getenv("TEMPORAL_GRPC_ENDPOINT"),
		Namespace: namespace,
		Logger:    NewNopLogger(),
	}

	tlsKeyPath := os.Getenv("TEMPORAL_TLS_KEY")
	tlsCertPath := os.Getenv("TEMPORAL_TLS_CERT")
	tlsCaPath := os.Getenv("TEMPORAL_TLS_CA")

	if tlsKeyPath != "" && tlsCertPath != "" {
		tlsConfig := tls.Config{}

		cert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		if err != nil {
			log.Fatalln("Unable to create key pair for TLS", err)
		}

		var tlsCaPool *x509.CertPool
		if tlsCaPath != "" {
			tlsCaPool = x509.NewCertPool()
			b, err := os.ReadFile(tlsCaPath)
			if err != nil {
				log.Fatalln("Failed reading server CA: %w", err)
			} else if !tlsCaPool.AppendCertsFromPEM(b) {
				log.Fatalln("Server CA PEM file invalid")
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

	var input []interface{}
	for _, a := range flag.Args() {
		var i interface{}
		err := json.Unmarshal([]byte(a), &i)
		if err != nil {
			log.Fatalln("Unable to parse input", err)
		}
		input = append(input, i)
	}

	c, err := client.Dial(clientOptions)
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	pool := pond.New(*nWorfklows, 0)

	go (func() {
		for {
			pool.Submit(func() {
				wf, err := c.ExecuteWorkflow(
					context.Background(),
					client.StartWorkflowOptions{
						TaskQueue: *sTaskQueue,
					},
					*sWorkflow,
					input...,
				)
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
