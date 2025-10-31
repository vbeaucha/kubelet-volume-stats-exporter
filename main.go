package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	defaultMetricsPort     = 8080
	defaultScrapeInterval  = 30 * time.Second
	defaultKubeletEndpoint = "https://127.0.0.1:10250"
)

var (
	// Command-line flags
	kubeletEndpoint = flag.String("kubelet-endpoint", defaultKubeletEndpoint, "Kubelet endpoint URL")
	metricsPort     = flag.Int("metrics-port", defaultMetricsPort, "Port to expose Prometheus metrics")
	scrapeInterval  = flag.Duration("scrape-interval", defaultScrapeInterval, "Interval to scrape kubelet stats")
	tokenPath       = flag.String("token-path", "/var/run/secrets/kubernetes.io/serviceaccount/token", "Path to service account token")
	insecureSkipTLS = flag.Bool("insecure-skip-tls-verify", false, "Skip TLS certificate verification")
	debugMode       = flag.Bool("debug", false, "Enable debug logging including raw API responses")

	// Prometheus metrics
	volumeCapacityBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_capacity_bytes",
			Help: "Capacity in bytes of the volume",
		},
		[]string{"namespace", "persistentvolumeclaim", "pod"},
	)

	volumeAvailableBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_available_bytes",
			Help: "Number of available bytes in the volume",
		},
		[]string{"namespace", "persistentvolumeclaim", "pod"},
	)

	volumeUsedBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_used_bytes",
			Help: "Number of used bytes in the volume",
		},
		[]string{"namespace", "persistentvolumeclaim", "pod"},
	)

	volumeInodesTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_inodes",
			Help: "Maximum number of inodes in the volume",
		},
		[]string{"namespace", "persistentvolumeclaim", "pod"},
	)

	volumeInodesFree = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_inodes_free",
			Help: "Number of free inodes in the volume",
		},
		[]string{"namespace", "persistentvolumeclaim", "pod"},
	)

	volumeInodesUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_inodes_used",
			Help: "Number of used inodes in the volume",
		},
		[]string{"namespace", "persistentvolumeclaim", "pod"},
	)

	scrapeErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "kubelet_volume_stats_scrape_errors_total",
			Help: "Total number of errors while scraping kubelet stats",
		},
	)

	lastScrapeTimestamp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "kubelet_volume_stats_last_scrape_timestamp_seconds",
			Help: "Timestamp of the last successful scrape",
		},
	)
)

// StatsResponse represents the kubelet /stats/summary response
type StatsResponse struct {
	Node NodeStats  `json:"node"`
	Pods []PodStats `json:"pods"`
}

type NodeStats struct {
	NodeName string `json:"nodeName"`
}

type PodStats struct {
	PodRef    PodReference  `json:"podRef"`
	Volume    []VolumeStats `json:"volume,omitempty"`
	Ephemeral *VolumeStats  `json:"ephemeral-storage,omitempty"` // Changed from array to pointer to object
}

type PodReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

type VolumeStats struct {
	Time           time.Time `json:"time"`
	Name           string    `json:"name"`
	PVCRef         *PVCRef   `json:"pvcRef,omitempty"`
	CapacityBytes  *uint64   `json:"capacityBytes,omitempty"`
	UsedBytes      *uint64   `json:"usedBytes,omitempty"`
	AvailableBytes *uint64   `json:"availableBytes,omitempty"`
	InodesTotal    *uint64   `json:"inodes,omitempty"`
	InodesFree     *uint64   `json:"inodesFree,omitempty"`
	InodesUsed     *uint64   `json:"inodesUsed,omitempty"`
}

type PVCRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type VolumeStatsCollector struct {
	client *http.Client
	token  string
	logger *zap.Logger
}

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(volumeCapacityBytes)
	prometheus.MustRegister(volumeAvailableBytes)
	prometheus.MustRegister(volumeUsedBytes)
	prometheus.MustRegister(volumeInodesTotal)
	prometheus.MustRegister(volumeInodesFree)
	prometheus.MustRegister(volumeInodesUsed)
	prometheus.MustRegister(scrapeErrorsTotal)
	prometheus.MustRegister(lastScrapeTimestamp)
}

func main() {
	flag.Parse()

	// Initialize logger with appropriate level
	var logger *zap.Logger
	var err error
	if *debugMode {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting kubelet volume stats exporter",
		zap.String("kubelet_endpoint", *kubeletEndpoint),
		zap.Int("metrics_port", *metricsPort),
		zap.Duration("scrape_interval", *scrapeInterval),
		zap.Bool("debug_mode", *debugMode),
	)

	// Read service account token
	token, err := readToken(*tokenPath)
	if err != nil {
		logger.Warn("Failed to read service account token, proceeding without authentication",
			zap.Error(err),
		)
	}

	// Create HTTP client
	tlsConfig := &tls.Config{
		InsecureSkipVerify: *insecureSkipTLS,
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	collector := &VolumeStatsCollector{
		client: client,
		token:  token,
		logger: logger,
	}

	// Start metrics collection in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.collectLoop(ctx)

	// Setup HTTP server for metrics
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *metricsPort),
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		logger.Info("Starting metrics server", zap.Int("port", *metricsPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start metrics server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gracefully...")

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during server shutdown", zap.Error(err))
	}

	logger.Info("Shutdown complete")
}

func (c *VolumeStatsCollector) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(*scrapeInterval)
	defer ticker.Stop()

	// Collect immediately on startup
	c.collectOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectOnce()
		}
	}
}

func (c *VolumeStatsCollector) collectOnce() {
	c.logger.Debug("Starting volume stats collection")

	stats, err := c.fetchStats()
	if err != nil {
		c.logger.Error("Failed to fetch stats",
			zap.Error(err),
			zap.String("kubelet_endpoint", *kubeletEndpoint),
		)
		scrapeErrorsTotal.Inc()
		return
	}

	c.logger.Debug("Successfully fetched stats, updating metrics",
		zap.Int("pod_count", len(stats.Pods)),
	)

	c.updateMetrics(stats)
	lastScrapeTimestamp.SetToCurrentTime()

	c.logger.Debug("Volume stats collection completed",
		zap.Int("pod_count", len(stats.Pods)),
	)
}

func (c *VolumeStatsCollector) fetchStats() (*StatsResponse, error) {
	url := fmt.Sprintf("%s/stats/summary", *kubeletEndpoint)

	c.logger.Debug("Fetching stats from kubelet", zap.String("url", url))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("Unexpected status code from kubelet",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response_body", string(body)),
		)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Read the entire response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log raw response in debug mode
	if *debugMode {
		c.logger.Debug("Raw kubelet API response",
			zap.String("response_body", string(bodyBytes)),
			zap.Int("response_size_bytes", len(bodyBytes)),
		)
	}

	// Unmarshal into a generic map first to inspect structure
	var rawResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &rawResponse); err != nil {
		c.logger.Error("Failed to unmarshal response as generic JSON",
			zap.Error(err),
			zap.String("response_preview", string(bodyBytes[:min(500, len(bodyBytes))])),
		)
		return nil, fmt.Errorf("failed to unmarshal generic JSON: %w", err)
	}

	// Log structure of pods array in debug mode
	if *debugMode {
		if pods, ok := rawResponse["pods"].([]interface{}); ok {
			c.logger.Debug("Found pods in response", zap.Int("pod_count", len(pods)))

			// Log ALL pods with their namespaces for debugging
			for i, podInterface := range pods {
				if pod, ok := podInterface.(map[string]interface{}); ok {
					// Extract podRef to see namespace
					if podRef, exists := pod["podRef"].(map[string]interface{}); exists {
						podName := ""
						podNamespace := ""
						podUID := ""

						if name, ok := podRef["name"].(string); ok {
							podName = name
						}
						if namespace, ok := podRef["namespace"].(string); ok {
							podNamespace = namespace
						}
						if uid, ok := podRef["uid"].(string); ok {
							podUID = uid
						}

						// Check if pod has volumes
						hasVolumes := false
						volumeCount := 0
						if volumes, exists := pod["volume"].([]interface{}); exists {
							volumeCount = len(volumes)
							hasVolumes = volumeCount > 0
						}

						c.logger.Debug("Pod from kubelet API",
							zap.Int("pod_index", i),
							zap.String("pod_name", podName),
							zap.String("pod_namespace", podNamespace),
							zap.String("pod_uid", podUID),
							zap.Bool("has_volumes", hasVolumes),
							zap.Int("volume_count", volumeCount),
						)
					}
				}
			}

			// Log first pod structure for inspection
			if len(pods) > 0 {
				if pod, ok := pods[0].(map[string]interface{}); ok {
					c.logger.Debug("First pod structure",
						zap.Any("pod_keys", getKeys(pod)),
					)

					// Check ephemeral-storage field specifically
					if ephemeral, exists := pod["ephemeral-storage"]; exists {
						c.logger.Debug("Found ephemeral-storage field",
							zap.String("type", fmt.Sprintf("%T", ephemeral)),
							zap.Any("value", ephemeral),
						)
					}

					// Check volume field
					if volume, exists := pod["volume"]; exists {
						c.logger.Debug("Found volume field",
							zap.String("type", fmt.Sprintf("%T", volume)),
						)
					}
				}
			}
		}
	}

	// Now unmarshal into our struct
	var stats StatsResponse
	if err := json.Unmarshal(bodyBytes, &stats); err != nil {
		c.logger.Error("Failed to decode response into StatsResponse struct",
			zap.Error(err),
			zap.String("response_preview", string(bodyBytes[:min(500, len(bodyBytes))])),
		)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Debug("Successfully parsed stats response",
		zap.Int("pod_count", len(stats.Pods)),
		zap.String("node_name", stats.Node.NodeName),
	)

	// Log all pods after unmarshaling to verify namespace values are preserved
	if *debugMode {
		c.logger.Debug("Pods after unmarshaling into Go struct:")
		for i, pod := range stats.Pods {
			c.logger.Debug("Pod from Go struct",
				zap.Int("pod_index", i),
				zap.String("pod_name", pod.PodRef.Name),
				zap.String("pod_namespace", pod.PodRef.Namespace),
				zap.String("pod_uid", pod.PodRef.UID),
				zap.Int("volume_count", len(pod.Volume)),
			)
		}
	}

	return &stats, nil
}

// Helper function to get keys from a map
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *VolumeStatsCollector) updateMetrics(stats *StatsResponse) {
	// Clear old metrics to avoid stale data
	volumeCapacityBytes.Reset()
	volumeAvailableBytes.Reset()
	volumeUsedBytes.Reset()
	volumeInodesTotal.Reset()
	volumeInodesFree.Reset()
	volumeInodesUsed.Reset()

	volumeCount := 0
	podCount := 0
	for _, pod := range stats.Pods {
		// Only process pods that have volumes with PVC references
		hasRelevantVolumes := false
		for _, vol := range pod.Volume {
			if vol.PVCRef != nil {
				hasRelevantVolumes = true
				break
			}
		}

		if hasRelevantVolumes {
			podCount++
			c.logger.Debug("Processing pod with PVC volumes",
				zap.String("pod", pod.PodRef.Name),
				zap.String("namespace", pod.PodRef.Namespace),
				zap.String("pod_uid", pod.PodRef.UID),
				zap.Int("volume_count", len(pod.Volume)),
			)
		} else if *debugMode {
			c.logger.Debug("Skipping pod without PVC volumes",
				zap.String("pod", pod.PodRef.Name),
				zap.String("namespace", pod.PodRef.Namespace),
				zap.Int("volume_count", len(pod.Volume)),
			)
		}

		for _, volume := range pod.Volume {
			// Only export metrics for volumes with PVC references
			if volume.PVCRef == nil {
				if *debugMode {
					c.logger.Debug("Skipping volume without PVC reference",
						zap.String("volume_name", volume.Name),
						zap.String("pod", pod.PodRef.Name),
						zap.String("pod_namespace", pod.PodRef.Namespace),
					)
				}
				continue
			}

			// Log the namespace value BEFORE creating labels
			c.logger.Debug("Creating metric labels for volume",
				zap.String("pod_name", pod.PodRef.Name),
				zap.String("pod_namespace_from_podref", pod.PodRef.Namespace),
				zap.String("pod_uid", pod.PodRef.UID),
				zap.String("pvc_name", volume.PVCRef.Name),
				zap.String("volume_name", volume.Name),
			)

			labels := prometheus.Labels{
				"namespace":             pod.PodRef.Namespace,
				"persistentvolumeclaim": volume.PVCRef.Name,
				"pod":                   pod.PodRef.Name,
			}

			// Log the labels AFTER creation to verify namespace is correct
			c.logger.Debug("Prometheus labels created",
				zap.String("label_namespace", labels["namespace"]),
				zap.String("label_pvc", labels["persistentvolumeclaim"]),
				zap.String("label_pod", labels["pod"]),
			)

			if volume.CapacityBytes != nil {
				volumeCapacityBytes.With(labels).Set(float64(*volume.CapacityBytes))
			}

			if volume.AvailableBytes != nil {
				volumeAvailableBytes.With(labels).Set(float64(*volume.AvailableBytes))
			}

			if volume.UsedBytes != nil {
				volumeUsedBytes.With(labels).Set(float64(*volume.UsedBytes))
			}

			if volume.InodesTotal != nil {
				volumeInodesTotal.With(labels).Set(float64(*volume.InodesTotal))
			}

			if volume.InodesFree != nil {
				volumeInodesFree.With(labels).Set(float64(*volume.InodesFree))
			}

			if volume.InodesUsed != nil {
				volumeInodesUsed.With(labels).Set(float64(*volume.InodesUsed))
			}

			volumeCount++
			c.logger.Debug("Updated metrics for volume",
				zap.String("namespace", pod.PodRef.Namespace),
				zap.String("pod", pod.PodRef.Name),
				zap.String("pvc", volume.PVCRef.Name),
				zap.String("volume_name", volume.Name),
				zap.Uint64p("capacity_bytes", volume.CapacityBytes),
				zap.Uint64p("used_bytes", volume.UsedBytes),
				zap.Uint64p("available_bytes", volume.AvailableBytes),
			)
		}
	}

	c.logger.Debug("Metrics update completed",
		zap.Int("pods_with_pvc_volumes", podCount),
		zap.Int("total_volumes_processed", volumeCount),
	)
}

func readToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}
