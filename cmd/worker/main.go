package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/temporalio/benchmark-workers-cadence/activities"
	"github.com/temporalio/benchmark-workers-cadence/cadenceclient"
	"github.com/temporalio/benchmark-workers-cadence/workflows"
	"go.uber.org/automaxprocs/maxprocs"

	"go.uber.org/cadence/activity"
	"go.uber.org/cadence/worker"
	"go.uber.org/cadence/workflow"
)

var sDomain = flag.String("n", "default", "domain")
var sTaskList = flag.String("tq", "benchmark", "task list")
var nMaxDecisionPollers = flag.Int("wp", -1, "max concurrent decision (workflow) task pollers (-1 = use Cadence default)")
var nMaxActivityPollers = flag.Int("ap", -1, "max concurrent activity task pollers (-1 = use Cadence default)")

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
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_DOMAIN\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_TASK_LIST\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_MAX_DECISION_TASK_POLLERS\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_MAX_ACTIVITY_TASK_POLLERS\n")
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
	domain := getStringValue("n", "CADENCE_DOMAIN", *sDomain, "default")
	taskList := getStringValue("tq", "CADENCE_TASK_LIST", *sTaskList, "benchmark")
	maxDecisionPollers := getIntValue("wp", "CADENCE_MAX_DECISION_TASK_POLLERS", *nMaxDecisionPollers, -1)
	maxActivityPollers := getIntValue("ap", "CADENCE_MAX_ACTIVITY_TASK_POLLERS", *nMaxActivityPollers, -1)

	log.Printf("Creating worker for domain: %s", domain)

	conn, err := cadenceclient.Dial(cadenceclient.Config{
		HostPort:                   os.Getenv("CADENCE_GRPC_ENDPOINT"),
		Domain:                     domain,
		Identity:                   "benchmark-worker",
		TLSKeyPath:                 os.Getenv("CADENCE_TLS_KEY"),
		TLSCertPath:                os.Getenv("CADENCE_TLS_CERT"),
		TLSCAPath:                  os.Getenv("CADENCE_TLS_CA"),
		TLSDisableHostVerification: os.Getenv("CADENCE_TLS_DISABLE_HOST_VERIFICATION") != "",
		PrometheusEndpoint:         os.Getenv("PROMETHEUS_ENDPOINT"),
	})
	if err != nil {
		log.Fatalf("Unable to create client: %v", err)
	}
	defer conn.Close()

	workerOptions := worker.Options{
		MetricsScope: conn.Scope,
		Logger:       conn.Logger,
	}

	// Cadence has no poller auto-scaling; these set a fixed poller count.
	// A value <= 0 leaves the Cadence SDK default in place.
	if maxDecisionPollers > 0 {
		workerOptions.MaxConcurrentDecisionTaskPollers = maxDecisionPollers
	}
	if maxActivityPollers > 0 {
		workerOptions.MaxConcurrentActivityTaskPollers = maxActivityPollers
	}

	w, err := worker.NewV2(conn.Service, domain, taskList, workerOptions)
	if err != nil {
		log.Fatalf("Unable to create worker: %v", err)
	}

	w.RegisterWorkflowWithOptions(workflows.ExecuteActivityWorkflow, workflow.RegisterOptions{Name: "ExecuteActivity"})
	w.RegisterWorkflowWithOptions(workflows.ReceiveSignalWorkflow, workflow.RegisterOptions{Name: "ReceiveSignal"})
	w.RegisterWorkflowWithOptions(workflows.DSLWorkflow, workflow.RegisterOptions{Name: "DSL"})
	w.RegisterActivityWithOptions(activities.SleepActivity, activity.RegisterOptions{Name: "Sleep"})
	w.RegisterActivityWithOptions(activities.EchoActivity, activity.RegisterOptions{Name: "Echo"})

	log.Printf("Starting worker for domain: %s", domain)
	// Run blocks until the worker is interrupted (SIGINT/SIGTERM).
	if err := w.Run(); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
