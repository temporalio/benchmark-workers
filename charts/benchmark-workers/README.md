# Temporal Benchmark Workers

This Helm chart deploys Temporal benchmark workers for load testing and performance evaluation of a Temporal cluster.

## TL;DR

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers
```

## Introduction

This chart deploys two components:
1. **Benchmark Workers**: Temporal workers that execute activities and workflows for benchmarking
2. **Soak Test** (optional): A runner component that continuously creates workflows to generate load

## Prerequisites

- Kubernetes 1.16+
- Helm 3.8.0+
- A running Temporal cluster accessible from the Kubernetes cluster
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
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers
```

### From Local Chart

To install the chart from a local clone of this repository:

```bash
git clone https://github.com/temporalio/benchmark-workers.git
cd benchmark-workers
helm install benchmark-workers ./charts/benchmark-workers
```

## Configuration

The following table lists the configurable parameters for the benchmark-workers chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `ghcr.io/temporalio/benchmark-workers` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `temporal.grpcEndpoint` | Temporal frontend endpoint | `temporal-frontend.temporal:7233` |
| `temporal.namespace` | Temporal namespace | `default` |
| `temporal.taskQueue` | Task queue name | `benchmark` |
| `temporal.workflowTaskPollers` | Number of workflow task pollers | `16` |
| `temporal.activityTaskPollers` | Number of activity task pollers | `8` |
| `temporal.tls.enabled` | Enable TLS | `false` |
| `temporal.tls.key` | TLS key content (base64 encoded) | `""` |
| `temporal.tls.cert` | TLS certificate content (base64 encoded) | `""` |
| `temporal.tls.ca` | TLS CA certificate content (base64 encoded) | `""` |
| `temporal.tls.existingSecret` | Use existing Kubernetes secret for TLS | `""` |
| `temporal.tls.disableHostVerification` | Disable TLS host verification | `false` |
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
| `soakTest.enabled` | Enable soak test deployment | `true` |
| `soakTest.replicaCount` | Number of soak test pods | `1` |
| `soakTest.concurrentWorkflows` | Number of concurrent workflows | `10` |
| `soakTest.workflowType` | Workflow type to execute | `ExecuteActivity` |
| `soakTest.workflowArgs` | Arguments for the workflow | `{ "Count": 3, "Activity": "Echo", "Input": { "Message": "test" } }` |
| `soakTest.resources` | Resource requests and limits for soak test pods | `{}` |
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity for pod assignment | `{}` |

## TLS Configuration

To use TLS, set `temporal.tls.enabled` to `true` and either:

1. Provide the TLS materials in the values file (not recommended for production):

```yaml
temporal:
  tls:
    enabled: true
    key: <base64-encoded-key>
    cert: <base64-encoded-cert>
    ca: <base64-encoded-ca>
```

2. Create a secret manually and reference it:

```bash
kubectl create secret generic temporal-tls \
  --from-file=key=/path/to/key.pem \
  --from-file=cert=/path/to/cert.pem \
  --from-file=ca=/path/to/ca.pem
```

Then reference it in your values:

```yaml
temporal:
  tls:
    enabled: true
    existingSecret: "temporal-tls"
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

## Examples

### Deploy workers with increased pollers

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set temporal.workflowTaskPollers=32 \
  --set temporal.activityTaskPollers=16
```

### Deploy with a high load soak test

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set soakTest.concurrentWorkflows=50
```

### Deploy with TLS enabled

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set temporal.tls.enabled=true \
  --set-file temporal.tls.key=/path/to/key.pem \
  --set-file temporal.tls.cert=/path/to/cert.pem \
  --set-file temporal.tls.ca=/path/to/ca.pem
```

### Deploy with Prometheus metrics enabled

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

### Scale worker or soak test replicas

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set workers.replicaCount=3 \
  --set soakTest.replicaCount=2
``` 