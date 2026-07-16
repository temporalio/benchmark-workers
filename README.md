# benchmark-workers-cadence

Pre-written workflows and activities useful for benchmarking [Cadence](https://cadenceworkflow.io/).

This is a **one-time port** of [`benchmark-workers`](https://github.com/temporalio/benchmark-workers)
(which targets Temporal) to the Cadence Go client, so the same workloads can be run against a
Cadence cluster for a comparative Temporal vs Cadence benchmark. It is published as a separate
package (module `github.com/temporalio/benchmark-workers-cadence`, image
`benchmark-workers-cadence`, chart `benchmark-workers-cadence`) and is not intended to be kept in
sync with the Temporal version.

This worker can be used alongside Maru or other benchmarking tools to mimic different workloads.

Also included is a simple workflow runner which will keep a configurable number of workflow
executions running concurrently to provide load for testing, starting a new execution each time
one completes.

## Differences from the Temporal version

- **Terminology**: Temporal *namespace* → Cadence *domain*; Temporal *task queue* → Cadence *task list*.
- **Transport**: connects to the Cadence frontend over gRPC (default port `7833`) using a YARPC
  dispatcher and the Thrift→Proto compatibility adapter.
- **Mandatory timeouts**: Cadence requires an execution timeout on every workflow start and
  explicit activity/child-workflow timeouts. The runner exposes `CADENCE_EXECUTION_TIMEOUT`
  (default 3600s); the workflows set sensible activity timeouts internally.
- **Pollers**: Cadence has no poller auto-scaling. `CADENCE_MAX_DECISION_TASK_POLLERS` and
  `CADENCE_MAX_ACTIVITY_TASK_POLLERS` set **fixed** poller counts (unset = SDK default).

## Usage

### Worker

The worker is available as a docker image for use in Docker or Kubernetes setups.

You can pull the latest image from: `ghcr.io/temporalio/benchmark-workers-cadence:cadence`.

The worker can be configured via environment variables. Currently only a small number of options
are available, please let us know if there is a particular option you would like to be exposed by
filing an issue.

| Environment Variable | Description |
| --- | --- |
| CADENCE_GRPC_ENDPOINT | The Cadence frontend gRPC endpoint (host:port, e.g. `cadence-frontend.cadence:7833`) |
| CADENCE_TLS_KEY | Path to TLS Key file |
| CADENCE_TLS_CERT | Path to TLS Cert file |
| CADENCE_TLS_CA | Path to TLS CA Cert file |
| CADENCE_TLS_DISABLE_HOST_VERIFICATION | If set, skip TLS host verification |
| CADENCE_DOMAIN | The Cadence domain |
| CADENCE_TASK_LIST | The Cadence task list |
| CADENCE_MAX_DECISION_TASK_POLLERS | Fixed number of decision (workflow) task pollers |
| CADENCE_MAX_ACTIVITY_TASK_POLLERS | Fixed number of activity task pollers |
| PROMETHEUS_ENDPOINT | The address to serve prometheus metrics on |

#### Kubernetes Deployment

There are several ways to deploy the worker in Kubernetes:

1. **Using kubectl run**:

```
kubectl run benchmark-worker --image ghcr.io/temporalio/benchmark-workers-cadence:cadence \
    --image-pull-policy Always \
    --env "CADENCE_GRPC_ENDPOINT=cadence-frontend.cadence:7833" \
    --env "CADENCE_DOMAIN=default" \
    --env "CADENCE_TASK_LIST=benchmark"
```

2. **Using the example deployment YAML**:

We provide an [example deployment spec](./deployment.yaml) for you to customize to your
requirements. Once you have edited the environment variables in the deployment.yaml you can create
the deployment with `kubectl apply -f ./deployment.yaml`.

3. **Using the Helm chart (Recommended)**:

We provide a Helm chart that can be installed from the GitHub Container Registry:

```bash
# Install the chart
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence
```

For more details and configuration options, see the [Helm chart documentation](./charts/benchmark-workers-cadence/README.md).

#### Prometheus Metrics

The worker can expose Prometheus metrics to help monitor the performance of your Cadence workers
and cluster. To enable metrics:

1. **Using kubectl or deployment YAML**:
   ```
   --env "PROMETHEUS_ENDPOINT=:9090"
   ```

2. **Using the Helm chart**:
   ```bash
   helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
     --set metrics.enabled=true
   ```

When using the Helm chart, it will automatically create a headless service for service discovery
and can optionally create a ServiceMonitor resource for Prometheus Operator:

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

You can then use the benchmark workflows with your benchmark tool. To test with the `cadence` CLI
you could run:

```
cadence --domain default workflow start \
  --tasklist benchmark \
  --workflow_type ExecuteActivity \
  --execution_timeout 60 \
  --input '{"Count":1,"Activity":"Sleep","Input":{"SleepTimeInSeconds":3}}'
```

This will run the ExecuteActivity workflow, described below.

### Runner

The runner is a tool that starts a set number of workflows concurrently and as each workflow
completes it will start another. This is useful for providing consistent load to your Cadence
cluster. The runner will start and maintain exactly the number of workflows concurrently that you
specified.

The runner can be configured via environment variables and command line arguments. Currently only
a small number of options are available, please let us know if there is a particular option you
would like to be exposed by filing an issue.

| Environment Variable | Description |
| --- | --- |
| CADENCE_GRPC_ENDPOINT | The Cadence frontend gRPC endpoint |
| CADENCE_TLS_KEY | Path to TLS Key file |
| CADENCE_TLS_CERT | Path to TLS Cert file |
| CADENCE_TLS_CA | Path to TLS CA Cert file |
| CADENCE_TLS_DISABLE_HOST_VERIFICATION | If set, skip TLS host verification |
| CADENCE_DOMAIN | The Cadence domain |
| CADENCE_TASK_LIST | The Cadence task list |
| CADENCE_CONCURRENT_WORKFLOWS | Number of concurrent workflows to maintain |
| CADENCE_WORKFLOW_TYPE | Workflow type to start |
| CADENCE_SIGNAL_TYPE | Signal type (for signal-with-start workflows) |
| CADENCE_WAIT | Whether to wait for each workflow to complete |
| CADENCE_EXECUTION_TIMEOUT | Workflow execution start-to-close timeout (seconds, default 3600) |
| CADENCE_DECISION_TASK_TIMEOUT | Decision task start-to-close timeout (seconds, 0 = Cadence default of 10s) |
| PROMETHEUS_ENDPOINT | The address to serve prometheus metrics on |
| CADENCE_DISABLE_ERROR_BACKOFF | Disable request exponential backoff on work request failure |
| CADENCE_BACKOFF_MAX_INTERVAL | Sets the max interval (seconds) that can be reached by the backoff |
| CADENCE_BACKOFF_FACTOR | Sets the factor the interval is multiplied by |

The runner is also configured via command line options:

```
Usage: runner [flags] [workflow input] ...
  -c int
    	concurrent workflows (default 10)
  -decision-timeout int
    	decision task start-to-close timeout (seconds, 0 = Cadence default)
  -execution-timeout int
    	workflow execution start-to-close timeout (seconds) (default 3600)
  -n string
    	domain (default "default")
  -s string
    	signal type
  -t string
    	workflow type
  -tq string
    	task list (default "benchmark")
  -w	wait for workflows to complete (default true)
```

To use the runner in a Kubernetes cluster you could use:

```
kubectl run benchmark-runner --image ghcr.io/temporalio/benchmark-workers-cadence:cadence \
    --image-pull-policy Always \
    --env "CADENCE_GRPC_ENDPOINT=cadence-frontend.cadence:7833" \
    --env "CADENCE_DOMAIN=default" \
    --command -- runner -t ExecuteActivity '{ "Count": 3, "Activity": "Echo", "Input": { "Message": "test" } }'
```

## Workflows

The worker provides the following workflows for you to use during benchmarking:

### ExecuteActivity

`ExecuteActivity({ Count: int, Activity: string, Input: interface{} })`

This workflow takes a count, an activity name and an activity input. The activity `Activity` will
be run `Count` times with the given `input`. If the activity returns an error the workflow will
fail with that error.

### ReceiveSignal

`ReceiveSignal()`

This workflow waits to receive a signal. It can be used with the runner's signal functionality to
test signal-based workflows.

### DSL

`DSL([]DSLStep)`

This workflow takes an array of steps, each of which can execute an activity or a child workflow
(which is another invocation of the DSL workflow). This allows you to compose complex benchmarking
scenarios, including nested and repeated activities and child workflows.

Each step can have the following fields:
- `a`: (string) Activity name to execute
- `i`: (object, optional) Input to pass to the activity
- `c`: (array of steps, optional) Child steps to execute as a child workflow
- `r`: (int, optional) Number of times to repeat this step (default 1)
- `p`: (int, optional) Size in bytes of padding data to add to activity inputs for increasing history size
- `t`: (int, optional) Seconds to sleep via a durable timer (`workflow.Sleep`) instead of the `Sleep` activity. A value of 0 (the default) is a no-op.

#### Examples

This example runs the `Echo` activity 3 times, then starts a child workflow which also runs the
`Echo` activity 3 times:

```
[
  {"a": "Echo", "i": {"Message": "test"}, "r": 3},
  {"c": [
    {"a": "Echo", "i": {"Message": "test"}, "r": 3}
  ]}
]
```

This example demonstrates using padding to increase history size by adding padding data to each
activity:

```
[
  {"a": "Echo", "i": {"Message": "test"}, "p": 1024},
  {"a": "Sleep", "i": {"SleepTimeInSeconds": 1}, "p": 2048},
  {"c": [
    {"a": "Echo", "i": {"Message": "nested"}, "p": 512}
  ]}
]
```

This example sleeps for 1 second using a durable timer (`workflow.Sleep`, no activity), runs `Echo`, then sleeps 5 seconds:

```
[
  {"t": 1},
  {"a": "Echo", "i": {"Message": "test"}},
  {"t": 5}
]
```

You can start this workflow using the `cadence` CLI or any Cadence client, for example:

```
cadence --domain default workflow start \
  --tasklist benchmark \
  --workflow_type DSL \
  --execution_timeout 60 \
  --input '[{"a": "Echo", "i": {"Message": "test"}, "r": 3}, {"c": [{"a": "Echo", "i": {"Message": "test"}, "r": 3}]}]'
```

## Activities

The worker provides the following activities for you to use during benchmarking:

### Sleep

`Sleep({ SleepTimeInSeconds: int, Padding?: []byte })`

This activity sleeps for the given number of seconds. It never returns an error. This can be used
to simulate activities which take a while to complete. The optional `Padding` field can be used to
increase the size of the activity input in workflow history.

### Echo

`Echo({ Message: string, Padding?: []byte }) result`

This activity simply returns the message as it's result. This can be used for stress testing
polling with activities that return instantly. The optional `Padding` field can be used to
increase the size of the activity input in workflow history.
