# Cadence Benchmark Workers

This Helm chart deploys Cadence benchmark workers for load testing and performance evaluation of a Cadence cluster.

It is the Cadence port of the Temporal `benchmark-workers` chart, published as a separate chart
(`benchmark-workers-cadence`) so the two can be installed side by side for a comparative benchmark.

## TL;DR

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence
```

## Introduction

This chart deploys two components:
1. **Benchmark Workers**: Cadence workers that execute activities and workflows for benchmarking
2. **Soak Test** (optional): A runner component that continuously creates workflows to generate load

## Prerequisites

- Kubernetes 1.16+
- Helm 3.8.0+
- A running Cadence cluster accessible from the Kubernetes cluster (frontend reachable over gRPC)
- (Optional) Prometheus Operator for ServiceMonitor support

## Installing the Chart

### From OCI Registry (Recommended)

To install the chart from the GitHub Container Registry:

```bash
# Authenticate with GHCR (if needed)
# For public repositories, this step is optional
# For private repositories:
# echo $GITHUB_TOKEN | helm registry login ghcr.io -u $GITHUB_USERNAME --password-stdin

# Install the chart
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence
```

### From Local Chart

To install the chart from a local clone of this repository:

```bash
git clone --branch cadence https://github.com/temporalio/benchmark-workers.git
cd benchmark-workers
helm install benchmark-workers-cadence ./charts/benchmark-workers-cadence
```

## Configuration

The following table lists the configurable parameters for the benchmark-workers-cadence chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `ghcr.io/temporalio/benchmark-workers-cadence` |
| `image.tag` | Image tag | appVersion from Chart.yaml |
| `image.pullPolicy` | Image pull policy | `Always` |
| `cadence.grpcEndpoint` | Cadence frontend gRPC endpoint | `cadence-frontend.cadence:7833` |
| `cadence.domain` | Cadence domain | `default` |
| `cadence.taskList` | Task list name | `benchmark` |
| `cadence.maxDecisionTaskPollers` | Fixed decision (workflow) task poller count | SDK default |
| `cadence.maxActivityTaskPollers` | Fixed activity task poller count | SDK default |
| `cadence.tls.enabled` | Enable TLS | `false` |
| `cadence.tls.key` | TLS key content (base64 encoded) | `""` |
| `cadence.tls.cert` | TLS certificate content (base64 encoded) | `""` |
| `cadence.tls.ca` | TLS CA certificate content (base64 encoded) | `""` |
| `cadence.tls.existingSecret` | Use existing Kubernetes secret for TLS | `""` |
| `cadence.tls.disableHostVerification` | Disable TLS host verification | `false` |
| `metrics.enabled` | Enable Prometheus metrics | `true` |
| `metrics.port` | Port to expose metrics on | `9090` |
| `metrics.prometheusEndpoint` | Prometheus metrics endpoint | `:9090` |
| `metrics.service.annotations` | Annotations for the metrics service | `{}` |
| `metrics.serviceMonitor.enabled` | Enable ServiceMonitor for Prometheus Operator | `true` |
| `metrics.serviceMonitor.additionalLabels` | Additional labels for the ServiceMonitor | `{}` |
| `metrics.serviceMonitor.interval` | Scrape interval | `15s` |
| `metrics.serviceMonitor.scrapeTimeout` | Scrape timeout | `10s` |
| `workers.replicaCount` | Number of worker pods | `1` |
| `workers.resources` | Resource requests and limits for worker pods | `{}` |
| `additionalEnv` | Additional environment variables for worker pods | `[]` |
| `soakTest.enabled` | Enable soak test deployment | `true` |
| `soakTest.replicaCount` | Number of soak test pods | `1` |
| `soakTest.concurrentWorkflows` | Number of concurrent workflows | `10` |
| `soakTest.workflowType` | Workflow type to execute | `ExecuteActivity` |
| `soakTest.workflowArgs` | Arguments for the workflow | `{ "Count": 3, "Activity": "Echo", "Input": { "Message": "test" } }` |
| `soakTest.resources` | Resource requests and limits for soak test pods | `{}` |
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity for pod assignment | `{}` |

> **Note:** Cadence has no poller auto-scaling. `cadence.maxDecisionTaskPollers` /
> `cadence.maxActivityTaskPollers` set fixed poller counts; leave them unset to use the SDK default.

## TLS Configuration

To use TLS, set `cadence.tls.enabled` to `true` and either:

1. Provide the TLS materials in the values file (not recommended for production):

```yaml
cadence:
  tls:
    enabled: true
    key: <base64-encoded-key>
    cert: <base64-encoded-cert>
    ca: <base64-encoded-ca>
```

2. Create a secret manually and reference it:

```bash
kubectl create secret generic cadence-tls \
  --from-file=key=/path/to/key.pem \
  --from-file=cert=/path/to/cert.pem \
  --from-file=ca=/path/to/ca.pem
```

Then reference it in your values:

```yaml
cadence:
  tls:
    enabled: true
    existingSecret: "cadence-tls"
```

## Prometheus Metrics Integration

### Basic Metrics

To enable basic metrics exposure:

```yaml
metrics:
  enabled: true
  port: 9090
  prometheusEndpoint: ":9090"
```

This will:
1. Configure the workers to expose Prometheus metrics
2. Create a headless service to make the metrics endpoints discoverable

### Prometheus Operator Integration

If you have the Prometheus Operator installed in your cluster, you can enable automatic service discovery:

```yaml
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    # Optional: Add custom labels for the Prometheus instance you want to use
    additionalLabels:
      release: monitoring
```

## Additional Environment Variables

The chart supports adding custom environment variables to the worker pods using the `additionalEnv` parameter. This is useful for configuring application-specific settings or integrating with external services.

### Simple Environment Variables

```yaml
additionalEnv:
  - name: CUSTOM_SETTING
    value: "my-value"
  - name: LOG_LEVEL
    value: "DEBUG"
```

### Environment Variables from Secrets

```yaml
additionalEnv:
  - name: DATABASE_PASSWORD
    valueFrom:
      secretKeyRef:
        name: my-secret
        key: password
```

## Examples

### Deploy workers with a fixed activity poller count

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
  --set cadence.maxActivityTaskPollers=150
```

### Deploy with a high load soak test

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
  --set soakTest.concurrentWorkflows=50
```

### Deploy with TLS enabled

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
  --set cadence.tls.enabled=true \
  --set-file cadence.tls.key=/path/to/key.pem \
  --set-file cadence.tls.cert=/path/to/cert.pem \
  --set-file cadence.tls.ca=/path/to/ca.pem
```

### Deploy with Prometheus metrics enabled

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

### Scale worker or soak test replicas

```bash
helm install benchmark-workers-cadence oci://ghcr.io/temporalio/charts/benchmark-workers-cadence \
  --set workers.replicaCount=3 \
  --set soakTest.replicaCount=2
```
