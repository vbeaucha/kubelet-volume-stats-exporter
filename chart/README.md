# Kubelet Volume Stats Exporter Helm Chart

A Helm chart for deploying the Kubelet Volume Stats Exporter, which restores volume statistics metrics in Kubernetes 1.34+ where `kubelet_volume_stats_*` metrics are no longer exposed by the kubelet.

## TL;DR

```bash
# Add the Helm repository
helm repo add vbeaucha https://vbeaucha.github.io/helm-charts
helm repo update

# Install the chart
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace
```

## Introduction

This chart deploys the Kubelet Volume Stats Exporter as a DaemonSet on a Kubernetes cluster using the Helm package manager. The exporter collects volume statistics from the kubelet's `/stats/summary` API and exposes them in Prometheus format.

### Problem Statement

Starting with Kubernetes 1.34, CSI volume statistics disappeared from both the kubelet `/stats/summary` and `/metrics` endpoints (see [kubernetes/kubernetes#133961](https://github.com/kubernetes/kubernetes/issues/133961)). This breaks monitoring and alerting for persistent volume usage.

### Solution

This exporter runs on every node as a DaemonSet, queries the local kubelet API, and exposes volume metrics with the same naming convention as the original `kubelet_volume_stats_*` metrics.

## Prerequisites

- Kubernetes 1.20+
- Helm 3.0+

**Note**: The chart uses `docker.io/vbeaucha/kubelet-volume-stats-exporter` by default.

## Installing the Chart

### Basic Installation

```bash
# Add the Helm repository
helm repo add vbeaucha https://vbeaucha.github.io/helm-charts
helm repo update

# Install the chart
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace
```

### Installation with Custom Values

Create a `custom-values.yaml` file:

```yaml
image:
  repository: docker.io/vbeaucha/kubelet-volume-stats-exporter
  tag: v1.0.0
  pullPolicy: IfNotPresent

config:
  scrapeInterval: "60s"
  insecureSkipTLSVerify: false

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

serviceMonitor:
  enabled: true
  interval: 60s
```

Install with custom values:

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace \
  -f custom-values.yaml
```

### Installation with Private Registry

```bash
# Create image pull secret
kubectl create secret docker-registry regcred \
  --docker-server=your-registry.com \
  --docker-username=your-username \
  --docker-password=your-password \
  -n kubelet-volume-stats

# Install with image pull secret
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --set image.repository=your-registry.com/kubelet-volume-stats-exporter \
  --set image.tag=v1.0.0 \
  --set imagePullSecrets[0].name=regcred
```

## Uninstalling the Chart

```bash
helm uninstall kubelet-volume-stats -n kubelet-volume-stats
```

This removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

The following table lists the configurable parameters of the chart and their default values.

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `kubelet-volume-stats-exporter` |
| `image.tag` | Container image tag | `""` (uses appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Application Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.kubeletEndpoint` | Kubelet API endpoint URL | `https://127.0.0.1:10250` |
| `config.metricsPort` | Port to expose Prometheus metrics | `8080` |
| `config.scrapeInterval` | Interval to scrape kubelet stats | `30s` |
| `config.insecureSkipTLSVerify` | Skip TLS certificate verification | `true` |
| `config.tokenPath` | Path to service account token | `/var/run/secrets/kubernetes.io/serviceaccount/token` |

### ServiceAccount Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name | `""` (generated) |

### RBAC Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |

### DaemonSet Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `daemonset.updateStrategy` | Update strategy | `RollingUpdate` |
| `daemonset.podAnnotations` | Pod annotations | Prometheus scrape annotations |
| `daemonset.podLabels` | Additional pod labels | `{}` |
| `daemonset.hostNetwork` | Use host network | `true` |
| `daemonset.hostPID` | Use host PID namespace | `false` |
| `daemonset.priorityClassName` | Priority class name | `system-node-critical` |

### Security Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.capabilities.drop` | Capabilities to drop | `[ALL]` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |
| `securityContext.runAsNonRoot` | Run as non-root user | `true` |
| `securityContext.runAsUser` | User ID | `1000` |

### Resource Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `50m` |
| `resources.requests.memory` | Memory request | `64Mi` |

### Scheduling Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nodeSelector` | Node selector | `{kubernetes.io/os: linux}` |
| `tolerations` | Tolerations | Run on all nodes |
| `affinity` | Affinity rules | `{}` |

### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.headless` | Create headless service | `true` |
| `service.annotations` | Service annotations | Prometheus scrape annotations |

### ServiceMonitor Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceMonitor.enabled` | Create ServiceMonitor | `false` |
| `serviceMonitor.namespace` | ServiceMonitor namespace | Release namespace |
| `serviceMonitor.labels` | ServiceMonitor labels | `{}` |
| `serviceMonitor.annotations` | ServiceMonitor annotations | `{}` |
| `serviceMonitor.interval` | Scrape interval | `30s` |
| `serviceMonitor.scrapeTimeout` | Scrape timeout | `""` |
| `serviceMonitor.metricRelabelings` | Metric relabel configs | See below |
| `serviceMonitor.relabelings` | Relabel configs | `[]` |


### Volume Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `volumes.token.expirationSeconds` | Token expiration seconds | `3607` |

## Examples

### Example 1: Basic Installation

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace
```

### Example 2: Enable Prometheus Operator Integration

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace \
  --set serviceMonitor.enabled=true
```

### Example 3: Custom Resource Limits for Large Clusters

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace \
  --set resources.limits.cpu=500m \
  --set resources.limits.memory=256Mi \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi
```

### Example 4: Disable TLS Verification (Production - Use CA Cert Instead)

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace \
  --set config.insecureSkipTLSVerify=false
```

### Example 5: Custom Scrape Interval

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace \
  --set config.scrapeInterval=60s
```

## Upgrading the Chart

### Upgrade to New Version

```bash
# Update repository
helm repo update

# Upgrade to latest version
helm upgrade kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats
```

### Upgrade with New Values

```bash
helm upgrade kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  -f custom-values.yaml
```

### Rollback to Previous Version

```bash
# List releases
helm history kubelet-volume-stats -n kubelet-volume-stats

# Rollback to specific revision
helm rollback kubelet-volume-stats 1 -n kubelet-volume-stats
```

## Verification

### Check DaemonSet Status

```bash
kubectl get daemonset -n kubelet-volume-stats
kubectl get pods -n kubelet-volume-stats -o wide
```

### View Logs

```bash
kubectl logs -n kubelet-volume-stats -l app.kubernetes.io/name=kubelet-volume-stats-exporter --tail=50
```

### Test Metrics Endpoint

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n kubelet-volume-stats daemonset/kubelet-volume-stats-exporter 8080:8080

# In another terminal, fetch metrics
curl http://localhost:8080/metrics | grep kubelet_volume_stats
```

### Verify Prometheus Scraping

If ServiceMonitor is enabled:

```bash
kubectl get servicemonitor -n kubelet-volume-stats
```

Check Prometheus targets to ensure the exporter is being scraped.

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl get pods -n kubelet-volume-stats

# Describe pod for events
kubectl describe pod -n kubelet-volume-stats <pod-name>

# Check logs
kubectl logs -n kubelet-volume-stats <pod-name>
```

### No Metrics Appearing

1. **Check if there are PVCs in the cluster:**
   ```bash
   kubectl get pvc --all-namespaces
   ```

2. **Test kubelet connectivity from pod:**
   ```bash
   kubectl exec -n kubelet-volume-stats -it <pod-name> -- sh
   wget -O- --no-check-certificate https://127.0.0.1:10250/stats/summary
   ```

3. **Check for scrape errors:**
   ```bash
   curl http://localhost:8080/metrics | grep scrape_errors_total
   ```

### RBAC Permission Errors

```bash
# Verify RBAC resources
kubectl get clusterrole kubelet-volume-stats-exporter
kubectl get clusterrolebinding kubelet-volume-stats-exporter

# Test permissions
kubectl auth can-i get nodes/stats \
  --as=system:serviceaccount:kubelet-volume-stats:kubelet-volume-stats-exporter
```

### Namespace Label Shows as `exported_namespace` in Grafana

**Symptom**: In Grafana/Prometheus, the namespace label appears as `exported_namespace` instead of `namespace`.

**Cause**: Prometheus automatically adds a `namespace` label from Kubernetes metadata (the exporter's namespace), which conflicts with the metric's own `namespace` label (the pod's namespace). Prometheus renames the metric's label to `exported_namespace` to avoid the conflict.

**Solution**: Enable ServiceMonitor with the default `metricRelabelings` configuration:

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --set serviceMonitor.enabled=true
```

The ServiceMonitor includes `metricRelabelings` that automatically rename `exported_namespace` back to `namespace`.

**Verification**:
```bash
# Check ServiceMonitor configuration
kubectl get servicemonitor -n kubelet-volume-stats -o yaml | grep -A 10 metricRelabelings

# Query Prometheus (adjust namespace if needed)
kubectl port-forward -n monitoring svc/prometheus-k8s 9090:9090 &
curl -s 'http://localhost:9090/api/v1/query?query=kubelet_volume_stats_capacity_bytes' | \
  jq '.data.result[0].metric | keys'
# Should include "namespace", NOT "exported_namespace"
```

## Metrics Exposed

The exporter exposes the following metrics:

- `kubelet_volume_stats_capacity_bytes` - Total volume capacity in bytes
- `kubelet_volume_stats_available_bytes` - Available space in bytes
- `kubelet_volume_stats_used_bytes` - Used space in bytes
- `kubelet_volume_stats_inodes` - Total number of inodes
- `kubelet_volume_stats_inodes_free` - Number of free inodes
- `kubelet_volume_stats_inodes_used` - Number of used inodes
- `kubelet_volume_stats_scrape_errors_total` - Total number of scrape errors
- `kubelet_volume_stats_last_scrape_timestamp_seconds` - Timestamp of last successful scrape

All metrics include the following labels:
- `namespace` - Kubernetes namespace
- `persistentvolumeclaim` - PVC name
- `pod` - Pod name

## Contributing

Contributions are welcome! Please see the main project repository for contribution guidelines.

## License

This chart is licensed under the MIT License. See the LICENSE file in the project root for details.

## Local Development

For local development and testing with the chart source code:

```bash
# Clone the repository
git clone https://github.com/vbeaucha/kubelet-volume-stats-exporter.git
cd kubelet-volume-stats-exporter

# Install from local chart directory
helm install kubelet-volume-stats-exporter ./chart \
  -n kubelet-volume-stats \
  --create-namespace

# Upgrade from local chart directory
helm upgrade kubelet-volume-stats-exporter ./chart \
  -n kubelet-volume-stats
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/vbeaucha/kubelet-volume-stats-exporter/issues
- Documentation: https://github.com/vbeaucha/kubelet-volume-stats-exporter
- Kubernetes Issue: https://github.com/kubernetes/kubernetes/issues/133961

