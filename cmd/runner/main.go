package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/alitto/pond"
	"github.com/uber-go/tally/v4/prometheus"
	sdktally "go.temporal.io/sdk/contrib/tally"

	"go.temporal.io/sdk/client"
)

var nWorfklows = flag.Int("c", 10, "concurrent workflows")
var sWorkflow = flag.String("t", "", "workflow type")
var bWait = flag.Bool("w", false, "wait for workflows to complete")
var sWorfklowID = flag.String("id", "benchmark", "workflow ID prefix")
var sTaskQueue = flag.String("tq", "benchmark", "task queue")

func main() {
	flag.Parse()

	clientOptions := client.Options{
		HostPort:  os.Getenv("TEMPORAL_GRPC_ENDPOINT"),
		Namespace: os.Getenv("TEMPORAL_NAMESPACE"),
		Logger:    NewNopLogger(),
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
		for i := 1; ; i++ {
			j := i
			pool.Submit(func() {
				wf, err := c.ExecuteWorkflow(
					context.Background(),
					client.StartWorkflowOptions{
						ID:        fmt.Sprintf("%s-%d", *sWorfklowID, j),
						TaskQueue: *sTaskQueue,
					},
					*sWorkflow,
					input...,
				)
				if err != nil {
					log.Fatalln("Unable to start workflow", err)
				}

				if *bWait {
					err = wf.Get(context.Background(), nil)
					if err != nil {
						log.Fatalln("Workflow failed", err)
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
