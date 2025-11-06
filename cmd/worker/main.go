package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

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

var sNamespace = flag.String("n", "default", "namespace")
var sTaskQueue = flag.String("tq", "benchmark", "task queue")
var nMaxWorkflowPollers = flag.Int("wp", -1, "max concurrent workflow task pollers (-1 = use default, 0 = disable)")
var nMaxActivityPollers = flag.Int("ap", -1, "max concurrent activity task pollers (-1 = use default, 0 = disable)")

// Track which flags were explicitly set
var flagsSet = make(map[string]bool)

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

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nEnvironment variables (used if flag not set):\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_NAMESPACE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_TASK_QUEUE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_MAX_WORKFLOW_TASK_POLLERS\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  TEMPORAL_MAX_ACTIVITY_TASK_POLLERS\n")
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
	namespace := getStringValue("n", "TEMPORAL_NAMESPACE", *sNamespace, "default")
	taskQueue := getStringValue("tq", "TEMPORAL_TASK_QUEUE", *sTaskQueue, "benchmark")
	maxWorkflowPollers := getIntValue("wp", "TEMPORAL_MAX_WORKFLOW_TASK_POLLERS", *nMaxWorkflowPollers, -1)
	maxActivityPollers := getIntValue("ap", "TEMPORAL_MAX_ACTIVITY_TASK_POLLERS", *nMaxActivityPollers, -1)

	log.Printf("Creating worker for namespace: %s", namespace)

	clientOptions := client.Options{
		HostPort:  os.Getenv("TEMPORAL_GRPC_ENDPOINT"),
		Namespace: namespace,
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

	workerOptions := worker.Options{}

	if maxWorkflowPollers >= 0 {
		workerOptions.WorkflowTaskPollerBehavior = worker.NewPollerBehaviorAutoscaling(worker.PollerBehaviorAutoscalingOptions{
			MaximumNumberOfPollers: maxWorkflowPollers,
		})
	} else {
		workerOptions.WorkflowTaskPollerBehavior = worker.NewPollerBehaviorSimpleMaximum(worker.PollerBehaviorSimpleMaximumOptions{})
	}

	if maxActivityPollers >= 0 {
		workerOptions.ActivityTaskPollerBehavior = worker.NewPollerBehaviorAutoscaling(worker.PollerBehaviorAutoscalingOptions{
			MaximumNumberOfPollers: maxActivityPollers,
		})
	} else {
		workerOptions.ActivityTaskPollerBehavior = worker.NewPollerBehaviorSimpleMaximum(worker.PollerBehaviorSimpleMaximumOptions{})
	}

	// TODO: Support more worker options

	w := worker.New(c, taskQueue, workerOptions)

	w.RegisterWorkflowWithOptions(workflows.ExecuteActivityWorkflow, workflow.RegisterOptions{Name: "ExecuteActivity"})
	w.RegisterWorkflowWithOptions(workflows.ReceiveSignalWorkflow, workflow.RegisterOptions{Name: "ReceiveSignal"})
	w.RegisterWorkflowWithOptions(workflows.DSLWorkflow, workflow.RegisterOptions{Name: "DSL"})
	w.RegisterActivityWithOptions(activities.SleepActivity, activity.RegisterOptions{Name: "Sleep"})
	w.RegisterActivityWithOptions(activities.EchoActivity, activity.RegisterOptions{Name: "Echo"})

	log.Printf("Starting worker for namespace: %s", namespace)
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
