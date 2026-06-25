package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/alitto/pond"
	"github.com/pborman/uuid"
	"github.com/temporalio/benchmark-workers-cadence/cadenceclient"
	"go.uber.org/automaxprocs/maxprocs"

	"go.uber.org/cadence/client"
)

var (
	nWorkflows        = flag.Int("c", 10, "concurrent workflows")
	sWorkflow         = flag.String("t", "", "workflow type")
	sSignalType       = flag.String("s", "", "signal type")
	bWait             = flag.Bool("w", true, "wait for workflows to complete")
	sDomain           = flag.String("n", "default", "domain")
	sTaskList         = flag.String("tq", "benchmark", "task list")
	nExecutionTimeout = flag.Int("execution-timeout", 3600, "workflow execution start-to-close timeout (seconds)")
	nDecisionTimeout  = flag.Int("decision-timeout", 0, "decision task start-to-close timeout (seconds, 0 = Cadence default)")
	nMaxInterval      = flag.Int("max-interval", 60, "maximum interval (in seconds) for exponential backoff")
	nFactor           = flag.Int("backoff-factor", 2, "factor for exponential backoff")
	bDisableBackoff   = flag.Bool("disable-backoff", false, "disable exponential backoff on errors")
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
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_CONCURRENT_WORKFLOWS\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_WORKFLOW_TYPE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_SIGNAL_TYPE\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_WAIT\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_DOMAIN\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_TASK_LIST\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  CADENCE_EXECUTION_TIMEOUT\n")
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
	concurrentWorkflows := getIntValue("c", "CADENCE_CONCURRENT_WORKFLOWS", *nWorkflows, 10)
	workflowType := getStringValue("t", "CADENCE_WORKFLOW_TYPE", *sWorkflow, "")
	signalType := getStringValue("s", "CADENCE_SIGNAL_TYPE", *sSignalType, "")
	waitForCompletion := getBoolValue("w", "CADENCE_WAIT", *bWait, true)
	domain := getStringValue("n", "CADENCE_DOMAIN", *sDomain, "default")
	taskList := getStringValue("tq", "CADENCE_TASK_LIST", *sTaskList, "benchmark")
	disableBackOff := getBoolValue("disable-backoff", "CADENCE_DISABLE_ERROR_BACKOFF", *bDisableBackoff, false)
	maxInterval := getIntValue("max-interval", "CADENCE_BACKOFF_MAX_INTERVAL", *nMaxInterval, 60)
	factor := getIntValue("backoff-factor", "CADENCE_BACKOFF_FACTOR", *nFactor, 2)
	executionTimeout := getIntValue("execution-timeout", "CADENCE_EXECUTION_TIMEOUT", *nExecutionTimeout, 3600)
	decisionTimeout := getIntValue("decision-timeout", "CADENCE_DECISION_TASK_TIMEOUT", *nDecisionTimeout, 0)

	log.Printf("Using domain: %s", domain)

	conn, err := cadenceclient.Dial(cadenceclient.Config{
		HostPort:                   os.Getenv("CADENCE_GRPC_ENDPOINT"),
		Domain:                     domain,
		Identity:                   "benchmark-runner",
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
	c := conn.Client

	log.Printf("Created client for domain: %s", domain)

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

	// Cadence requires an explicit execution timeout (and defaults the decision
	// task timeout to 10s) on every start.
	startOpts := client.StartWorkflowOptions{
		TaskList:                     taskList,
		ExecutionStartToCloseTimeout: time.Duration(executionTimeout) * time.Second,
	}
	if decisionTimeout > 0 {
		startOpts.DecisionTaskStartToCloseTimeout = time.Duration(decisionTimeout) * time.Second
	}

	var starter func() (client.WorkflowRun, error)

	if signalType != "" {
		starter = func() (client.WorkflowRun, error) {
			wID := uuid.New()
			opts := startOpts
			opts.ID = wID
			// SignalWithStartWorkflow returns only the execution identifiers in
			// Cadence; re-resolve a WorkflowRun via GetWorkflow so we can wait.
			exec, err := c.SignalWithStartWorkflow(
				context.Background(),
				wID,
				signalType,
				nil,
				opts,
				workflowType,
				input...,
			)
			if err != nil {
				return nil, err
			}
			return c.GetWorkflow(context.Background(), exec.ID, exec.RunID), nil
		}
	} else {
		starter = func() (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(
				context.Background(),
				startOpts,
				workflowType,
				input...,
			)
		}
	}

	go (func() {
		currentInterval := 1
		errChan := make(chan error, concurrentWorkflows)

		for {
			pool.Submit(func() {
				wf, err := starter()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Unable to start workflow: %v\n", err)
					errChan <- err
					return
				}

				if waitForCompletion {
					err = wf.Get(context.Background(), nil)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Workflow failed: %v\n", err)
						errChan <- err
						return
					}
				}

				errChan <- nil
			})

			var lastErr error
			updated := false

		drainLoop:
			for {
				select {
				case err := <-errChan:
					lastErr = err
					updated = true
				default:
					break drainLoop
				}
			}

			if disableBackOff || !updated {
				continue
			}

			if lastErr != nil {
				fmt.Fprintf(os.Stderr, "Waiting for %d seconds before retrying to start workflow...\n", currentInterval)
				time.Sleep(time.Duration(currentInterval) * time.Second)
				nInterval := currentInterval * factor
				if nInterval < maxInterval && maxInterval != 0 {
					currentInterval *= factor
				}
			} else if lastErr == nil {
				currentInterval = 1
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
