# benchmark-workers

Pre-written workflows and activities useful for benchmarking Temporal.

This worker can be used alongside Maru or other benchmarking tools to mimic different workloads.

Note: Only the basic infrastructure and one workflow is currently included. This will be expanded in the near future.

## Usage

The worker is available as docker image for use in Docker or Kubernetes setups.

You can pull the latest image from: `ghcr.io/temporalio/benchmark-workers:main`.

In future we will provide releases with appropriate image tags to make benchmarks more easily repeatable.

The worker can be configured via environment variables. Currently only a small number of options are available, please let us know if there is a particular option you would like to be exposed by filing an issue.

The table below lists the environment variables available and the relevant Temporal Go SDK options they relate to (the worker is currently written in Go).

| Environment Variable | Relevant Client or Worker option | Description |
| --- | --- | --- |
| TEMPORAL_GRPC_ENDPOINT | [ClientOptions.HostPort](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ClientOptions) | The Temporal Frontend GRPC endpoint |
| TEMPORAL_NAMESPACE | [ClientOptions.Namespace](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ClientOptions) | The Temporal Namespace |
| TEMPORAL_TASK_QUEUE | [TaskQueue](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/worker#New) | The Temporal Task Queue |
| TEMPORAL_WORKFLOW_TASK_POLLERS | [WorkerOptions.MaxConcurrentWorkflowTaskPollers](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#WorkerOptions) | Number of workflow task pollers |
| TEMPORAL_ACTIVITY_TASK_POLLERS | [WorkerOptions.MaxConcurrentActivityTaskPollers](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#WorkerOptions) | Number of activity task pollers |
| PROMETHEUS_ENDPOINT | n/a | The address to serve prometheus metrics on |

To run the worker in a Kubernetes cluster you could use:

```
kubectl run benchmark-worker --image ghcr.io/temporalio/benchmark-workers:main \
    --image-pull-policy Always \
    --env "TEMPORAL_GRPC_ENDPOINT=temporal-frontend.temporal:7233" \
    --env "TEMPORAL_NAMESPACE=default" \
    --env "TEMPORAL_TASK_QUEUE=benchmark" \
    --env "TEMPORAL_WORKFLOW_TASK_POLLERS=16" \
    --env "TEMPORAL_WORKFLOW_ACTIVITY_TASK_POLLERS=8"
```

Note: If you need more than one worker to test performance at scale you should use a Kubernetes deployment instead of using `kubectl run` which creates only one pod. We provide an [example deployment spec](./deployment.yaml) for you to customize to your requirements. Once you have edited the environment variables in the deployment.yaml you can create the deployment with `kubectl apply -f ./deployment.yaml`.

You can then use the benchmark workflows with your benchmark tool. To test with `tctl` you could run:

```
tctl config set version 2
tctl workflow start --task-queue benchmark --type ExecuteActivity --execution-timeout 60 --input '{"Count":1,"Activity":"Sleep","Input":{"SleepTimeInSeconds":3}}'
```

This will run the ExecuteActivity workflow, described below.

## Workflows

The worker provides the following workflows for you to use during benchmarking:

### ExecuteActivity

`ExecuteActivity({ Count: int, Activity: string, Input: interface{} })`

This workflow takes a count, an activity name and an activity input. The activity `Activity` will be run `Count` times with the given `input`. If the activity returns an error the workflow will fail with that error.

## Activities

The worker provides the following activities for you to use during benchmarking:

### Sleep

`Sleep({ SleepTimeInSeconds: int })`

This activity sleeps for the given number of seconds. It never returns an error. This can be used to simulate activities which take a while to complete.

### Echo

`Echo({ Message: string }) result`

This activity simply returns the message as it's result. This can be used for stress testing polling with activities that return instantly.