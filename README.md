# benchmark-workers

Pre-written workflows and activities useful for benchmarking Temporal.

This worker can be used alongside Maru or other benchmarking tools to mimic different workloads.

Also included is a simple workflow runner which will keep a configurable number of workflow executions running concurrently to provide load for testing, starting a new execution each time one completes.

## Usage

### Worker

The worker is available as docker image for use in Docker or Kubernetes setups.

You can pull the latest image from: `ghcr.io/temporalio/benchmark-workers:main`.

In future we will provide releases with appropriate image tags to make benchmarks more easily repeatable.

The worker can be configured via environment variables. Currently only a small number of options are available, please let us know if there is a particular option you would like to be exposed by filing an issue.

The table below lists the environment variables available and the relevant Temporal Go SDK options they relate to (the worker is currently written using the Temporal Go SDK).

| Environment Variable | Relevant Client or Worker option | Description |
| --- | --- | --- |
| TEMPORAL_GRPC_ENDPOINT | [ClientOptions.HostPort](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ClientOptions) | The Temporal Frontend GRPC endpoint (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TLS_KEY | [ClientOptions.ConnectionOptions.TLS](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ConnectionOptions) | Path to TLS Key file (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TLS_CERT | [ClientOptions.ConnectionOptions.TLS](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ConnectionOptions) | Path to TLS Cert file (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TLS_CA | [ClientOptions.ConnectionOptions.TLS](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ConnectionOptions) | Path to TLS CA Cert file (supports comma-separated values for multiple namespaces) |
| TEMPORAL_NAMESPACE | [ClientOptions.Namespace](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ClientOptions) | The Temporal Namespace (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TASK_QUEUE | [TaskQueue](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/worker#New) | The Temporal Task Queue |
| TEMPORAL_WORKFLOW_TASK_POLLERS | [WorkerOptions.MaxConcurrentWorkflowTaskPollers](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#WorkerOptions) | Number of workflow task pollers |
| TEMPORAL_ACTIVITY_TASK_POLLERS | [WorkerOptions.MaxConcurrentActivityTaskPollers](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#WorkerOptions) | Number of activity task pollers |
| PROMETHEUS_ENDPOINT | n/a | The address to serve prometheus metrics on |

#### Multi-Namespace Support

The worker supports working with multiple namespaces simultaneously. This allows you to spread load across multiple namespaces with a single worker deployment, providing a more realistic load pattern.

**Configuration Options:**

1. **Same configuration for all namespaces:**
   ```bash
   export TEMPORAL_NAMESPACE="ns1,ns2,ns3"
   export TEMPORAL_GRPC_ENDPOINT="temporal.example.com:7233"
   export TEMPORAL_TLS_CERT="/path/to/cert.pem"
   export TEMPORAL_TLS_KEY="/path/to/key.pem"
   ```

2. **Different configurations per namespace:**
   ```bash
   export TEMPORAL_NAMESPACE="ns1,ns2,ns3"
   export TEMPORAL_GRPC_ENDPOINT="temporal1.example.com:7233,temporal2.example.com:7233,temporal3.example.com:7233"
   export TEMPORAL_TLS_CERT="/certs/ns1.pem,/certs/ns2.pem,/certs/ns3.pem"
   export TEMPORAL_TLS_KEY="/keys/ns1.key,/keys/ns2.key,/keys/ns3.key"
   ```

3. **Mixed configuration (some shared, some different):**
   ```bash
   export TEMPORAL_NAMESPACE="ns1,ns2,ns3"
   export TEMPORAL_GRPC_ENDPOINT="temporal.example.com:7233"  # Same endpoint for all
   export TEMPORAL_TLS_CERT="/certs/ns1.pem,/certs/ns2.pem"  # ns3 will reuse ns2's cert
   export TEMPORAL_TLS_KEY="/keys/shared.key"                # Same key for all
   ```

**How it works:**
- If you provide exactly as many values as namespaces, each namespace uses its corresponding value
- If you provide only one value but multiple namespaces, that single value is reused for all namespaces
- If you provide fewer values than namespaces, the last value is repeated for the remaining namespaces

The worker will create a separate worker instance for each namespace, all running concurrently within the same process.

#### Kubernetes Deployment

There are several ways to deploy the worker in Kubernetes:

1. **Using kubectl run**:

```
kubectl run benchmark-worker --image ghcr.io/temporalio/benchmark-workers:main \
    --image-pull-policy Always \
    --env "TEMPORAL_GRPC_ENDPOINT=temporal-frontend.temporal:7233" \
    --env "TEMPORAL_NAMESPACE=default" \
    --env "TEMPORAL_TASK_QUEUE=benchmark" \
    --env "TEMPORAL_WORKFLOW_TASK_POLLERS=16" \
    --env "TEMPORAL_ACTIVITY_TASK_POLLERS=8"
```

2. **Multi-namespace deployment example**:

```
kubectl run benchmark-worker --image ghcr.io/temporalio/benchmark-workers:main \
    --image-pull-policy Always \
    --env "TEMPORAL_GRPC_ENDPOINT=temporal-frontend.temporal:7233" \
    --env "TEMPORAL_NAMESPACE=namespace1,namespace2,namespace3" \
    --env "TEMPORAL_TASK_QUEUE=benchmark" \
    --env "TEMPORAL_WORKFLOW_TASK_POLLERS=16" \
    --env "TEMPORAL_ACTIVITY_TASK_POLLERS=8"
```

3. **Using the example deployment YAML**:

We provide an [example deployment spec](./deployment.yaml) for you to customize to your requirements. Once you have edited the environment variables in the deployment.yaml you can create the deployment with `kubectl apply -f ./deployment.yaml`.

4. **Using the Helm chart (Recommended)**:

We provide a Helm chart that can be installed from the GitHub Container Registry:

```bash
# Install the chart
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers
```

For more details and configuration options, see the [Helm chart documentation](./charts/benchmark-workers/README.md).

#### Prometheus Metrics

The worker can expose Prometheus metrics to help monitor the performance of your Temporal workers and cluster. To enable metrics:

1. **Using kubectl or deployment YAML**:
   ```
   --env "PROMETHEUS_ENDPOINT=:9090"
   ```

2. **Using the Helm chart**:
   ```bash
   helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
     --set metrics.enabled=true
   ```

When using the Helm chart, it will automatically create a headless service for service discovery and can optionally create a ServiceMonitor resource for Prometheus Operator:

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

You can then use the benchmark workflows with your benchmark tool. To test with `tctl` you could run:

```
tctl workflow start --taskqueue benchmark --workflow_type ExecuteActivity --execution_timeout 60 -i '{"Count":1,"Activity":"Sleep","Input":{"SleepTimeInSeconds":3}}'
```

This will run the ExecuteActivity workflow, described below.

### Runner

The runner is available as docker image for use in Docker or Kubernetes setups.

You can pull the latest image from: `ghcr.io/temporalio/benchmark-workers:main`.

The runner can be configured via environment variables and command line arguments. Currently only a small number of options are available, please let us know if there is a particular option you would like to be exposed by filing an issue.

The table below lists the environment variables available and the relevant Temporal Go SDK options they relate to (the runner is currently written using the Temporal Go SDK).

| Environment Variable | Relevant Client or Worker option | Description |
| --- | --- | --- |
| TEMPORAL_GRPC_ENDPOINT | [ClientOptions.HostPort](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ClientOptions) | The Temporal Frontend GRPC endpoint (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TLS_KEY | [ClientOptions.ConnectionOptions.TLS.Certificates](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ConnectionOptions) | Path to TLS Key file (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TLS_CERT | [ClientOptions.ConnectionOptions.TLS.Certificates](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ConnectionOptions) | Path to TLS Cert file (supports comma-separated values for multiple namespaces) |
| TEMPORAL_TLS_CA | [ClientOptions.ConnectionOptions.TLS](https://pkg.go.dev/go.temporal.io/sdk@v1.15.0/internal#ConnectionOptions) | Path to TLS CA Cert file (supports comma-separated values for multiple namespaces) |
| PROMETHEUS_ENDPOINT | n/a | The address to serve prometheus metrics on |

The runner is also configured via command line options:

```
Usage: runner [flags] [workflow input] ...
  -c int
    	concurrent workflows (default 10)
  -n string
    	namespace (comma-separated list supported) (default "default")
  -s string
    	signal type
  -t string
    	workflow type
  -tq string
    	task queue (default "benchmark")
  -w	wait for workflows to complete (default true)
```

#### Multi-Namespace Support in Runner

The runner supports distributing workflow executions across multiple namespaces. This provides a more realistic load pattern by spreading the load across different namespaces.

**Configuration Examples:**

1. **Command line flag for multiple namespaces:**
   ```bash
   runner -n "namespace1,namespace2,namespace3" -t ExecuteActivity '{"Count":1,"Activity":"Sleep","Input":{"SleepTimeInSeconds":3}}'
   ```

2. **Environment variable for multiple namespaces:**
   ```bash
   export TEMPORAL_NAMESPACE="namespace1,namespace2,namespace3"
   runner -t ExecuteActivity '{"Count":1,"Activity":"Sleep","Input":{"SleepTimeInSeconds":3}}'
   ```

3. **Different GRPC endpoints per namespace:**
   ```bash
   export TEMPORAL_NAMESPACE="ns1,ns2,ns3"
   export TEMPORAL_GRPC_ENDPOINT="temporal1.example.com:7233,temporal2.example.com:7233,temporal3.example.com:7233"
   runner -t ExecuteActivity '{"Count":1,"Activity":"Sleep","Input":{"SleepTimeInSeconds":3}}'
   ```

**How it works:**
- The runner creates a separate client for each namespace
- Workflow executions are distributed across namespaces using round-robin rotation
- Each client can have different connection settings (endpoints, TLS certificates, etc.)

To use the runner in a Kubernetes cluster you could use:

```
kubectl run benchmark-runner --image ghcr.io/temporalio/benchmark-workers:main \
    --image-pull-policy Always \
    --env "TEMPORAL_GRPC_ENDPOINT=temporal-frontend.temporal:7233" \
    --env "TEMPORAL_NAMESPACE=namespace1,namespace2,namespace3" \
    --command -- runner -t ExecuteActivity '{ "Count": 3, "Activity": "Echo", "Input": { "Message": "test" } }'
```

## Workflows

The worker provides the following workflows for you to use during benchmarking:

### ExecuteActivity

`ExecuteActivity({ Count: int, Activity: string, Input: interface{} })`

This workflow takes a count, an activity name and an activity input. The activity `Activity` will be run `Count` times with the given `input`. If the activity returns an error the workflow will fail with that error.

### ReceiveSignal

`ReceiveSignal()`

This workflow waits to receive a signal. It can be used with the runner's signal functionality to test signal-based workflows.

### DSLWorkflow

`DSLWorkflow([]DSLStep)`

This workflow takes an array of steps, each of which can execute an activity or a child workflow (which is another invocation of DSLWorkflow). This allows you to compose complex benchmarking scenarios, including nested and repeated activities and child workflows.

Each step can have the following fields:
- `a`: (string) Activity name to execute
- `i`: (object, optional) Input to pass to the activity
- `c`: (array of steps, optional) Child steps to execute as a child workflow
- `r`: (int, optional) Number of times to repeat this step (default 1)

#### Example

This example runs the `Echo` activity 3 times, then starts a child workflow which also runs the `Echo` activity 3 times:

```
[
  {"a": "Echo", "i": {"Message": "test"}, "r": 3},
  {"c": [
    {"a": "Echo", "i": {"Message": "test"}, "r": 3}
  ]}
]
```

You can start this workflow using `tctl` or any Temporal client, for example:

```
tctl workflow start --taskqueue benchmark --workflow_type DSLWorkflow --execution_timeout 60 -i '[{"a": "Echo", "i": {"Message": "test"}, "r": 3}, {"c": [{"a": "Echo", "i": {"Message": "test"}, "r": 3}]}]'
```

## Activities

The worker provides the following activities for you to use during benchmarking:

### Sleep

`Sleep({ SleepTimeInSeconds: int })`

This activity sleeps for the given number of seconds. It never returns an error. This can be used to simulate activities which take a while to complete.

### Echo

`Echo({ Message: string }) result`

This activity simply returns the message as it's result. This can be used for stress testing polling with activities that return instantly.