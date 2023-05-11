package main

import (
	"crypto/tls"
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

func main() {
	if _, err := maxprocs.Set(); err != nil {
		log.Printf("WARNING: failed to set GOMAXPROCS: %v.\n", err)
	}

	clientOptions := client.Options{
		HostPort:  os.Getenv("TEMPORAL_GRPC_ENDPOINT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
	}
	if clientOptions.Namespace == "" {
		clientOptions.Namespace = "default"
	}

	tlsKeyPath := os.Getenv("TEMPORAL_TLS_KEY")
	tlsCertPath := os.Getenv("TEMPORAL_TLS_CERT")

	if tlsKeyPath != "" && tlsCertPath != "" {
		cert, err := tls.LoadX509KeyPair(tlsCertPath, tlsKeyPath)
		if err != nil {
			log.Fatalln("Unable to create key pair for TLS", err)
		}

		clientOptions.ConnectionOptions.TLS = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
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

	if os.Getenv("TEMPORAL_WORKFLOW_TASK_POLLERS") != "" {
		i, err := strconv.Atoi(os.Getenv("TEMPORAL_WORKFLOW_TASK_POLLERS"))
		if err != nil {
			log.Fatalf("TEMPORAL_WORKFLOW_TASK_POLLERS is invalid: %v", err)
		}
		workerOptions.MaxConcurrentWorkflowTaskPollers = i
	}

	if os.Getenv("TEMPORAL_ACTIVITY_TASK_POLLERS") != "" {
		i, err := strconv.Atoi(os.Getenv("TEMPORAL_ACTIVITY_TASK_POLLERS"))
		if err != nil {
			log.Fatalf("TEMPORAL_ACTIVITY_TASK_POLLERS is invalid: %v", err)
		}
		workerOptions.MaxConcurrentActivityTaskPollers = i
	}

	// TODO: Support more worker options

	taskQueue := os.Getenv("TEMPORAL_TASK_QUEUE")
	if taskQueue == "" {
		taskQueue = "benchmark"
	}
	w := worker.New(c, taskQueue, workerOptions)

	w.RegisterWorkflowWithOptions(workflows.ExecuteActivityWorkflow, workflow.RegisterOptions{Name: "ExecuteActivity"})
	w.RegisterActivityWithOptions(activities.SleepActivity, activity.RegisterOptions{Name: "Sleep"})
	w.RegisterActivityWithOptions(activities.EchoActivity, activity.RegisterOptions{Name: "Echo"})

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
