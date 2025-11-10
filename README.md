# Kubelet Volume Stats Exporter

A Kubernetes DaemonSet application that addresses the regression in Kubernetes 1.34 where `kubelet_volume_stats_*` metrics are no longer exposed by the kubelet. This exporter retrieves volume statistics from the kubelet's `/stats/summary` API endpoint and exposes them in Prometheus format for backward compatibility.

## Problem Statement

Starting with Kubernetes 1.34, CSI volume statistics disappeared from both the kubelet `/stats/summary` and `/metrics` endpoints (see [kubernetes/kubernetes#133961](https://github.com/kubernetes/kubernetes/issues/133961)). This breaks monitoring and alerting for persistent volume usage across clusters.

## Solution

This application:
- Runs as a DaemonSet on every node in your cluster
- Queries the local kubelet's `/stats/summary` API endpoint
- Extracts volume statistics for all pods on the node
- Exposes metrics in Prometheus format with the same metric names as the original `kubelet_volume_stats_*` metrics
- Provides backward compatibility for existing monitoring dashboards and alerts

## Metrics Exposed

The exporter provides the following metrics with labels `namespace`, `persistentvolumeclaim`, and `pod`:

- `kubelet_volume_stats_capacity_bytes` - Capacity in bytes of the volume
- `kubelet_volume_stats_available_bytes` - Number of available bytes in the volume
- `kubelet_volume_stats_used_bytes` - Number of used bytes in the volume
- `kubelet_volume_stats_inodes` - Maximum number of inodes in the volume
- `kubelet_volume_stats_inodes_free` - Number of free inodes in the volume
- `kubelet_volume_stats_inodes_used` - Number of used inodes in the volume

Additional operational metrics:
- `kubelet_volume_stats_scrape_errors_total` - Total number of errors while scraping kubelet stats
- `kubelet_volume_stats_last_scrape_timestamp_seconds` - Timestamp of the last successful scrape

## Prerequisites

- Kubernetes cluster version 1.34+ (or any version where volume stats are missing)
- `kubectl` configured to access your cluster
- Docker or compatible container runtime for building the image
- (Optional) Prometheus Operator for ServiceMonitor support

## Quick Start

### Deploy with Helm

```bash
# Add the Helm repository
helm repo add vbeaucha https://vbeaucha.github.io/helm-charts
helm repo update

# Install the chart
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --create-namespace

# Verify deployment
kubectl get daemonset -n kubelet-volume-stats
kubectl get pods -n kubelet-volume-stats

# Test metrics endpoint
kubectl port-forward -n kubelet-volume-stats daemonset/kubelet-volume-stats-exporter 8080:8080
curl http://localhost:8080/metrics | grep kubelet_volume_stats
```

## Configuration

The exporter supports the following command-line flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--kubelet-endpoint` | `https://127.0.0.1:10250` | Kubelet endpoint URL |
| `--metrics-port` | `8080` | Port to expose Prometheus metrics |
| `--scrape-interval` | `30s` | Interval to scrape kubelet stats |
| `--token-path` | `/var/run/secrets/kubernetes.io/serviceaccount/token` | Path to service account token |
| `--insecure-skip-tls-verify` | `false` | Skip TLS certificate verification |

You can modify these in the DaemonSet manifest under the `args` section.

## Prometheus Integration

### Standard Prometheus

The Service includes annotations for automatic discovery:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

#### Complete Scrape Configuration

For standard Prometheus (non-Operator), add this scrape configuration to handle label conflicts:

```yaml
scrape_configs:
  - job_name: 'kubelet-volume-stats-exporter'
    kubernetes_sd_configs:
      - role: endpoints
        namespaces:
          names:
            - kubelet-volume-stats

    relabel_configs:
      # Keep only endpoints for the kubelet-volume-stats-exporter service
      - source_labels: [__meta_kubernetes_service_name]
        action: keep
        regex: kubelet-volume-stats-exporter

      # Use pod name as instance label
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: instance

      # Add node name label
      - source_labels: [__meta_kubernetes_pod_node_name]
        target_label: node

    # Fix label conflicts: Prometheus adds namespace/pod labels from Kubernetes metadata,
    # which conflict with the metric's own namespace/pod labels, causing them to be
    # renamed to exported_namespace/exported_pod. These rules rename them back.
    metric_relabel_configs:
      # Rename exported_namespace back to namespace
      - source_labels: [exported_namespace]
        target_label: namespace
        action: replace

      # Drop the exported_namespace label
      - regex: exported_namespace
        action: labeldrop

      # Rename exported_pod back to pod
      - source_labels: [exported_pod]
        target_label: pod
        action: replace

      # Drop the exported_pod label
      - regex: exported_pod
        action: labeldrop
```

**Why these relabeling rules are needed**: Prometheus automatically adds `namespace` and `pod` labels from Kubernetes service discovery metadata (the exporter's namespace/pod). These conflict with the metric's own `namespace` and `pod` labels (the PVC's namespace/pod), causing Prometheus to rename the metric labels to `exported_namespace` and `exported_pod`. The `metric_relabel_configs` above fix this by renaming them back.

### Prometheus Operator

Enable ServiceMonitor for automatic scraping:

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --set serviceMonitor.enabled=true
```

The ServiceMonitor includes the necessary `metricRelabelings` to automatically handle the label conflict issue described above.

### Label Conflict Issue: `exported_namespace` and `exported_pod`

**Symptom**: In Grafana or Prometheus, you see labels named `exported_namespace` and `exported_pod` instead of `namespace` and `pod`.

**Root Cause**: This is a common Prometheus label conflict issue:

1. **The exporter exports metrics** with labels: `namespace="default"` (the PVC's namespace) and `pod="my-app-pod"` (the pod using the PVC)
2. **Prometheus adds metadata labels** from Kubernetes service discovery: `namespace="kubelet-volume-stats"` (the exporter's namespace) and `pod="exporter-pod"` (the exporter pod)
3. **Conflict detected**: Two labels with the same name but different values
4. **Prometheus renames**: To avoid the conflict, Prometheus renames the metric's labels to `exported_namespace` and `exported_pod`
5. **Result**: Your queries and dashboards see the wrong label names

**Solution 1: Use ServiceMonitor (Recommended for Prometheus Operator)**

Enable ServiceMonitor which includes automatic label fixing:

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --set serviceMonitor.enabled=true
```

The ServiceMonitor includes `metricRelabelings` that automatically rename `exported_namespace` → `namespace` and `exported_pod` → `pod`.

**Solution 2: Manual Prometheus Configuration (Standard Prometheus)**

Add `metric_relabel_configs` to your Prometheus scrape configuration (see "Complete Scrape Configuration" section above).

**Verification**:

```bash
# Query Prometheus to check label names
curl -s 'http://prometheus:9090/api/v1/query?query=kubelet_volume_stats_capacity_bytes' | \
  jq '.data.result[0].metric | keys'

# Should include "namespace" and "pod", NOT "exported_namespace" or "exported_pod"
```

### Example Prometheus Queries

```promql
# Volume usage percentage
100 * (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes)

# Volumes with less than 10% free space
kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes < 0.1

# Total volume capacity by namespace
sum by (namespace) (kubelet_volume_stats_capacity_bytes)

# Volumes by pod
sum by (namespace, pod, persistentvolumeclaim) (kubelet_volume_stats_capacity_bytes)
```

## Troubleshooting

### Labels show as `exported_namespace` and `exported_pod`

See the "Label Conflict Issue" section under Prometheus Integration above.

### Exporter stops working after running for some time

**Symptom**: The exporter stops fetching volume statistics after running for 1 days. Restarting the pod temporarily fixes the issue.

**Root Cause**: Service account token expiration. Kubernetes service account tokens have a limited lifetime and are automatically rotated by the kubelet.

**Solution**: The exporter automatically handles token rotation by reloading the token from the filesystem on each request. This is enabled by default in version 1.1.0+.

**Configuration**:
- Token expiration is set to 24 hours by default (`volumes.token.expirationSeconds: 86400`)
- Kubernetes automatically rotates the token before expiration
- The exporter reads the token file on each kubelet API request, picking up the rotated token automatically

**Verification**:
```bash
# Check token file is being updated
kubectl exec -n kubelet-volume-stats daemonset/kubelet-volume-stats-exporter -- \
  stat /var/run/secrets/kubernetes.io/serviceaccount/token

# Check exporter logs for token-related messages
kubectl logs -n kubelet-volume-stats -l app.kubernetes.io/name=kubelet-volume-stats-exporter | \
  grep -i token

# Check for authentication errors
kubectl logs -n kubelet-volume-stats -l app.kubernetes.io/name=kubelet-volume-stats-exporter | \
  grep -i "401\|unauthorized\|forbidden"
```

**For older versions** (< 1.1.0): Upgrade to the latest version which includes automatic token refresh.

### High memory usage

Adjust resource limits in the DaemonSet:
```yaml
resources:
  limits:
    memory: 256Mi  # Increase if needed
```

### TLS certificate errors

If you encounter TLS certificate verification errors, you can enable insecure mode (not recommended for production):
```yaml
args:
  - --insecure-skip-tls-verify=true
```

## Security Considerations

The exporter follows Kubernetes security best practices:

- Runs as non-root user (UID 1000)
- Uses read-only root filesystem
- Drops all Linux capabilities
- Implements seccomp profile
- Uses service account tokens for authentication
- Minimal RBAC permissions (only access to node stats)

## Development

### Local Development

```bash
# Install dependencies
go mod download

# Run locally (requires kubeconfig)
go run main.go --kubelet-endpoint=https://your-node:10250 --insecure-skip-tls-verify=true

# Run tests
go test ./...

# Build binary
go build -o kubelet-volume-stats-exporter .
```

### Building for Multiple Architectures

```bash
# Build for AMD64
GOOS=linux GOARCH=amd64 go build -o kubelet-volume-stats-exporter-amd64 .

# Build for ARM64
GOOS=linux GOARCH=arm64 go build -o kubelet-volume-stats-exporter-arm64 .
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Kubernetes Node                      │
│                                                              │
│  ┌──────────────┐                    ┌──────────────────┐  │
│  │   Kubelet    │◄───────────────────│  Volume Stats    │  │
│  │              │  /stats/summary    │    Exporter      │  │
│  │  Port 10250  │                    │   (DaemonSet)    │  │
│  └──────────────┘                    └────────┬─────────┘  │
│                                                │             │
│                                                │ :8080       │
└────────────────────────────────────────────────┼────────────┘
                                                 │
                                                 │
                                    ┌────────────▼─────────────┐
                                    │      Prometheus          │
                                    │   (scrapes metrics)      │
                                    └──────────────────────────┘
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) file for details.

## References

- [Kubernetes Issue #133961](https://github.com/kubernetes/kubernetes/issues/133961) - CSI volume statistics missing
- [Kubelet Stats Summary API](https://kubernetes.io/docs/reference/instrumentation/node-metrics/)
- [Kubernetes Metrics Reference](https://kubernetes.io/docs/reference/instrumentation/metrics/)

