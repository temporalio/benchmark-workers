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
	"strconv"
	"time"

	"github.com/alitto/pond"
	"github.com/pborman/uuid"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"
	"go.uber.org/automaxprocs/maxprocs"

	"go.temporal.io/sdk/client"
)

var (
	nWorkflows  = flag.Int("c", 10, "concurrent workflows")
	sWorkflow = flag.String("t", "", "workflow type")
	sSignalType = flag.String("s", "", "signal type")
	bWait = flag.Bool("w", true, "wait for workflows to complete")
	sNamespace = flag.String("n", "default", "namespace")
	sTaskQueue = flag.String("tq", "benchmark", "task queue")
	nmaxInterval = flag.Int("max-interval", 60, "maximum interval (in seconds) for exponential backoff")
	nfactor = flag.Int("backoff-factor", 2, "factor for exponential backoff")
	bBackoff = flag.Bool("disable-backoff", false, "disable exponential backoff on errors")
	
)

// Track which flags were explicitly set
var flagsSet = make(map[string]bool)

// flagValue helps implement precedence: command line > environment variable > default
func getStringValue(flagName, envName, flagValue, defaultValue string) string {
	if flagsSet[flagName] {
		return flagValue
	}
	if envValue := os.Getenv(envName); envValue != "" {
		return envValue
	}
	return defaultValue
}

func getIntValue(flagName, envName string, flagValue, defaultValue int) int {
	if flagsSet[flagName] {
		return flagValue
	}
	if envValue := os.Getenv(envName); envValue != "" {
		if parsed, err := strconv.Atoi(envValue); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getBoolValue(flagName, envName string, flagValue, defaultValue bool) bool {
	if flagsSet[flagName] {
		return flagValue
	}
	if envValue := os.Getenv(envName); envValue != "" {
		if parsed, err := strconv.ParseBool(envValue); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [workflow input] ...\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nEnvironment variables (used if flag not set):\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_CONCURRENT_WORKFLOWS\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_WORKFLOW_TYPE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_SIGNAL_TYPE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_WAIT\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_NAMESPACE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_TASK_QUEUE\n")
	}

	flag.Parse()

	// Track which flags were explicitly set by the user
	flag.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
	})

	if _, err := maxprocs.Set(); err != nil {
		log.Printf("WARNING: failed to set GOMAXPROCS: %v.\n", err)
	}

	// Apply precedence: command line > environment variable > default
	concurrentWorkflows := getIntValue("c", "TEMPORAL_CONCURRENT_WORKFLOWS", *nWorkflows, 10)
	workflowType := getStringValue("t", "TEMPORAL_WORKFLOW_TYPE", *sWorkflow, "")
	signalType := getStringValue("s", "TEMPORAL_SIGNAL_TYPE", *sSignalType, "")
	waitForCompletion := getBoolValue("w", "TEMPORAL_WAIT", *bWait, true)
	namespace := getStringValue("n", "TEMPORAL_NAMESPACE", *sNamespace, "default")
	taskQueue := getStringValue("tq", "TEMPORAL_TASK_QUEUE", *sTaskQueue, "benchmark")
	backOff := getBoolValue("disable-backoff", "TEMPORAL_DISABLE_ERROR_BACKOFF", *bBackoff, false)
	maxInterval := getIntValue("max-interval", "TEMPORAL_BACKOFF_MAX_INTERVAL", *nmaxInterval, 60)
	factor := getIntValue("backoff-factor", "TEMPORAL_BACKOFF_FACTOR", *nfactor, 2)

	log.Printf("Using namespace: %s", namespace)

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
			log.Fatalf("Unable to create key pair for TLS: %v", err)
		}

		var tlsCaPool *x509.CertPool
		if tlsCaPath != "" {
			tlsCaPool = x509.NewCertPool()
			b, err := os.ReadFile(tlsCaPath)
			if err != nil {
				log.Fatalf("Failed reading server CA: %v", err)
			} else if !tlsCaPool.AppendCertsFromPEM(b) {
				log.Fatalf("Server CA PEM file invalid")
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
		log.Fatalf("Unable to create client: %v", err)
	}
	defer c.Close()

	log.Printf("Created client for namespace: %s", namespace)

	var input []interface{}
	for _, a := range flag.Args() {
		var i interface{}
		err := json.Unmarshal([]byte(a), &i)
		if err != nil {
			log.Fatalln("Unable to parse input", err)
		}
		input = append(input, i)
	}

	pool := pond.New(concurrentWorkflows, 0)

	var starter func() (client.WorkflowRun, error)

	if signalType != "" {
		starter = func() (client.WorkflowRun, error) {
			wID := uuid.New()
			return c.SignalWithStartWorkflow(
				context.Background(),
				wID,
				signalType,
				nil,
				client.StartWorkflowOptions{
					ID:        wID,
					TaskQueue: taskQueue,
				},
				workflowType,
				input...,
			)
		}
	} else {
		starter = func() (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(
				context.Background(),
				client.StartWorkflowOptions{
					TaskQueue: taskQueue,
				},
				workflowType,
				input...,
			)
		}
	}

	go (func() {
		var errorOccurred bool=false
		currentInterval := 1
		for {
			pool.Submit(func() {
				wf, err := starter()
				if err != nil {
					log.Println("Unable to start workflow", err)
					errorOccurred = true
					return
				}

				if waitForCompletion {
					err = wf.Get(context.Background(), nil)
					if err != nil {
						log.Println("Workflow failed", err)
						return
					}
				}
			})
			if errorOccurred  && !backOff{
				currentInterval *= factor

				if currentInterval > maxInterval {
					log.Println("Unable to start workflow after retries", err)
					os.Exit(1)
				}

				log.Printf("Waiting for %d seconds before retrying to start workflow...", currentInterval)
				time.Sleep(time.Duration(currentInterval) * time.Second)
				errorOccurred = false
			}
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
