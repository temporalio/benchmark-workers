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

## Multi-Namespace Support

The chart supports deploying workers and runners that work with multiple Temporal namespaces simultaneously. This provides a more realistic load pattern by distributing work across multiple namespaces with a single deployment.

### Configuration Examples

#### Single Namespace (Traditional)
```yaml
temporal:
  grpcEndpoint: "temporal-frontend.temporal:7233"
  namespace: "default"
```

#### Multiple Namespaces with Same Configuration
```yaml
temporal:
  grpcEndpoint: "temporal-frontend.temporal:7233"  # Same endpoint for all
  namespace: ["namespace1", "namespace2", "namespace3"]
```

#### Multiple Namespaces with Different Endpoints
```yaml
temporal:
  grpcEndpoint: ["temporal1.temporal:7233", "temporal2.temporal:7233", "temporal3.temporal:7233"]
  namespace: ["namespace1", "namespace2", "namespace3"]
```

#### Multiple Namespaces with Mixed TLS Configuration
```yaml
temporal:
  grpcEndpoint: ["temporal1.temporal:7233", "temporal2.temporal:7233"]
  namespace: ["namespace1", "namespace2", "namespace3"]
  tls:
    enabled: true
    # Use arrays for different TLS configs per namespace
    keys: ["key1-content", "key2-content"]  # namespace3 will reuse key2-content
    certs: ["cert1-content", "cert2-content"] # namespace3 will reuse cert2-content
    ca: "shared-ca-content"  # Same CA for all namespaces
```

## Configuration

The following table lists the configurable parameters for the benchmark-workers chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `ghcr.io/temporalio/benchmark-workers` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `temporal.grpcEndpoint` | Temporal frontend endpoint(s) (string or array) | `temporal-frontend.temporal:7233` |
| `temporal.namespace` | Temporal namespace(s) (string or array) | `default` |
| `temporal.taskQueue` | Task queue name | `benchmark` |
| `temporal.workflowTaskPollers` | Number of workflow task pollers | `16` |
| `temporal.activityTaskPollers` | Number of activity task pollers | `8` |
| `temporal.tls.enabled` | Enable TLS | `false` |
| `temporal.tls.key` | TLS key content (base64 encoded) | `""` |
| `temporal.tls.cert` | TLS certificate content (base64 encoded) | `""` |
| `temporal.tls.ca` | TLS CA certificate content (base64 encoded) | `""` |
| `temporal.tls.keys` | Array of TLS key contents for multi-namespace | `[]` |
| `temporal.tls.certs` | Array of TLS certificate contents for multi-namespace | `[]` |
| `temporal.tls.cas` | Array of TLS CA certificate contents for multi-namespace | `[]` |
| `temporal.tls.existingSecret` | Use existing Kubernetes secret for TLS | `""` |
| `temporal.tls.existingSecrets` | Array of existing Kubernetes secrets for multi-namespace | `[]` |
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

## TLS Configuration

### Single Namespace TLS

To use TLS with a single namespace, set `temporal.tls.enabled` to `true` and either:

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

### Multi-Namespace TLS

For multiple namespaces, you can either:

1. **Use the same TLS configuration for all namespaces:**

```yaml
temporal:
  namespace: ["ns1", "ns2", "ns3"]
  tls:
    enabled: true
    key: <base64-encoded-key>  # Same key for all namespaces
    cert: <base64-encoded-cert>  # Same cert for all namespaces
    ca: <base64-encoded-ca>  # Same CA for all namespaces
```

2. **Use different TLS configurations per namespace:**

```yaml
temporal:
  namespace: ["ns1", "ns2", "ns3"]
  tls:
    enabled: true
    keys: ["<base64-key1>", "<base64-key2>", "<base64-key3>"]
    certs: ["<base64-cert1>", "<base64-cert2>", "<base64-cert3>"]
    cas: ["<base64-ca1>", "<base64-ca2>", "<base64-ca3>"]
```

3. **Mix single and array values (arrays take precedence):**

```yaml
temporal:
  namespace: ["ns1", "ns2", "ns3"]
  tls:
    enabled: true
    keys: ["<base64-key1>", "<base64-key2>"]  # ns3 will reuse key2
    certs: ["<base64-cert1>", "<base64-cert2>"]  # ns3 will reuse cert2
    ca: "<base64-shared-ca>"  # Same CA for all namespaces
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
  - name: API_KEY
    valueFrom:
      secretKeyRef:
        name: api-credentials
        key: api-key
```

### Environment Variables from ConfigMaps

```yaml
additionalEnv:
  - name: APP_CONFIG
    valueFrom:
      configMapKeyRef:
        name: app-config
        key: config.json
```

### Mixed Environment Variables

```yaml
additionalEnv:
  - name: ENVIRONMENT
    value: "production"
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: database-credentials
        key: url
  - name: FEATURE_FLAGS
    valueFrom:
      configMapKeyRef:
        name: feature-config
        key: flags
```

## Examples

### Deploy workers with multiple namespaces

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set temporal.namespace="{namespace1,namespace2,namespace3}"
```

### Deploy with different endpoints per namespace

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set temporal.namespace="{namespace1,namespace2}" \
  --set temporal.grpcEndpoint="{temporal1.temporal:7233,temporal2.temporal:7233}"
```

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

### Deploy with TLS enabled for multiple namespaces

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set temporal.namespace="{namespace1,namespace2}" \
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

### Deploy with additional environment variables

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set additionalEnv[0].name=LOG_LEVEL \
  --set additionalEnv[0].value=DEBUG \
  --set additionalEnv[1].name=CUSTOM_SETTING \
  --set additionalEnv[1].value=production-value
```

### Scale worker or soak test replicas

```bash
helm install benchmark-workers oci://ghcr.io/temporalio/charts/benchmark-workers \
  --set workers.replicaCount=3 \
  --set soakTest.replicaCount=2
``` 