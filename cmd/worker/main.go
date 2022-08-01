package main

import (
	"log"
	"os"
	"strconv"

	"github.com/robholland/benchmark-workers/activities"
	"github.com/robholland/benchmark-workers/workflows"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	clientOptions := client.Options{
		HostPort:  os.Getenv("TEMPORAL_GRPC_ENDPOINT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
		// TODO: Support TLS.
	}

	if os.Getenv("PROMETHEUS_ENDPOINT") != "" {
		clientOptions.MetricsHandler = sdktally.NewMetricsHandler(newPrometheusScope(prometheus.Configuration{
			ListenAddress: os.Getenv("PROMETHEUS_ENDPOINT"),
			TimerType:     "histogram",
		}))
	}

	c, err := client.Dial(clientOptions)
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	workerOptions := worker.Options{}

	if os.Getenv("TEMPORAL_MAX_CONCURRENT_WORKFLOW_TASK_POLLERS") != "" {
		i, err := strconv.Atoi(os.Getenv("TEMPORAL_MAX_CONCURRENT_WORKFLOW_TASK_POLLERS"))
		if err != nil {
			log.Fatalf("TEMPORAL_MAX_CONCURRENT_WORKFLOW_TASK_POLLERS is invalid: %v", err)
		}
		workerOptions.MaxConcurrentWorkflowTaskPollers = i
	}

	if os.Getenv("TEMPORAL_MAX_CONCURRENT_ACTIVITY_TASK_POLLERS") != "" {
		i, err := strconv.Atoi(os.Getenv("TEMPORAL_MAX_CONCURRENT_ACTIVITY_TASK_POLLERS"))
		if err != nil {
			log.Fatalf("TEMPORAL_MAX_CONCURRENT_ACTIVITY_TASK_POLLERS is invalid: %v", err)
		}
		workerOptions.MaxConcurrentActivityTaskPollers = i
	}

	// TODO: Support more worker options

	w := worker.New(c, os.Getenv("TEMPORAL_TASK_QUEUE"), workerOptions)

	w.RegisterWorkflow(workflows.SerialWorkflow)
	w.RegisterActivity(activities.SleepActivity)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
