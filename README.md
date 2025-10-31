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

### Prometheus Operator

Enable ServiceMonitor for automatic scraping:

```bash
helm upgrade --install kubelet-volume-stats vbeaucha/kubelet-volume-stats-exporter \
  -n kubelet-volume-stats \
  --set serviceMonitor.enabled=true
```

### Example Prometheus Queries

```promql
# Volume usage percentage
100 * (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes)

# Volumes with less than 10% free space
kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes < 0.1

# Total volume capacity by namespace
sum by (namespace) (kubelet_volume_stats_capacity_bytes)
```

## Security Considerations

The exporter follows Kubernetes security best practices:

- Runs as non-root user (UID 1000)
- Uses read-only root filesystem
- Drops all Linux capabilities
- Implements seccomp profile
- Uses service account tokens for authentication
- Minimal RBAC permissions (only access to node stats)

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

