# Makefile for Helm chart operations

CHART_NAME := kubelet-volume-stats-exporter
CHART_VERSION := $(shell grep '^version:' Chart.yaml | awk '{print $$2}')
NAMESPACE := kubelet-volume-stats
RELEASE_NAME := kubelet-volume-stats-exporter

# Image configuration
IMAGE_REPOSITORY := docker.io/vbeaucha/kubelet-volume-stats-exporter
IMAGE_TAG := v1.0.0

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo "$(GREEN)Helm Chart Makefile for $(CHART_NAME)$(NC)"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

.PHONY: lint
lint: ## Lint the Helm chart
	@echo "$(YELLOW)Linting Helm chart...$(NC)"
	helm lint .
	@echo "$(GREEN)✓ Lint completed$(NC)"

.PHONY: template
template: ## Generate Kubernetes manifests from the chart
	@echo "$(YELLOW)Generating templates...$(NC)"
	helm template $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--set image.repository=$(IMAGE_REPOSITORY) \
		--set image.tag=$(IMAGE_TAG)
	@echo "$(GREEN)✓ Templates generated$(NC)"

.PHONY: template-debug
template-debug: ## Generate templates with debug output
	@echo "$(YELLOW)Generating templates with debug...$(NC)"
	helm template $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--set image.repository=$(IMAGE_REPOSITORY) \
		--set image.tag=$(IMAGE_TAG) \
		--debug

.PHONY: dry-run
dry-run: ## Perform a dry-run installation
	@echo "$(YELLOW)Performing dry-run installation...$(NC)"
	helm install $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set image.repository=$(IMAGE_REPOSITORY) \
		--set image.tag=$(IMAGE_TAG) \
		--dry-run --debug
	@echo "$(GREEN)✓ Dry-run completed$(NC)"

.PHONY: install
install: ## Install the Helm chart
	@echo "$(YELLOW)Installing Helm chart...$(NC)"
	helm install $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--create-namespace \
		--set image.repository=$(IMAGE_REPOSITORY) \
		--set image.tag=$(IMAGE_TAG)
	@echo "$(GREEN)✓ Chart installed$(NC)"

.PHONY: install-dev
install-dev: ## Install with development values
	@echo "$(YELLOW)Installing with development values...$(NC)"
	helm install $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--create-namespace \
		-f examples/values-development.yaml
	@echo "$(GREEN)✓ Chart installed (development)$(NC)"

.PHONY: install-prod
install-prod: ## Install with production values
	@echo "$(YELLOW)Installing with production values...$(NC)"
	helm install $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--create-namespace \
		-f examples/values-production.yaml
	@echo "$(GREEN)✓ Chart installed (production)$(NC)"

.PHONY: upgrade
upgrade: ## Upgrade the Helm release
	@echo "$(YELLOW)Upgrading Helm release...$(NC)"
	helm upgrade $(RELEASE_NAME) . \
		--namespace $(NAMESPACE) \
		--set image.repository=$(IMAGE_REPOSITORY) \
		--set image.tag=$(IMAGE_TAG)
	@echo "$(GREEN)✓ Chart upgraded$(NC)"

.PHONY: uninstall
uninstall: ## Uninstall the Helm release
	@echo "$(YELLOW)Uninstalling Helm release...$(NC)"
	helm uninstall $(RELEASE_NAME) --namespace $(NAMESPACE)
	@echo "$(GREEN)✓ Chart uninstalled$(NC)"

.PHONY: status
status: ## Show the status of the Helm release
	@helm status $(RELEASE_NAME) --namespace $(NAMESPACE)

.PHONY: list
list: ## List Helm releases
	@helm list --namespace $(NAMESPACE)

.PHONY: history
history: ## Show release history
	@helm history $(RELEASE_NAME) --namespace $(NAMESPACE)

.PHONY: rollback
rollback: ## Rollback to previous release
	@echo "$(YELLOW)Rolling back to previous release...$(NC)"
	helm rollback $(RELEASE_NAME) --namespace $(NAMESPACE)
	@echo "$(GREEN)✓ Rollback completed$(NC)"

.PHONY: get-values
get-values: ## Get values of the deployed release
	@helm get values $(RELEASE_NAME) --namespace $(NAMESPACE)

.PHONY: get-manifest
get-manifest: ## Get manifest of the deployed release
	@helm get manifest $(RELEASE_NAME) --namespace $(NAMESPACE)

.PHONY: package
package: ## Package the Helm chart
	@echo "$(YELLOW)Packaging Helm chart...$(NC)"
	helm package .
	@echo "$(GREEN)✓ Chart packaged: $(CHART_NAME)-$(CHART_VERSION).tgz$(NC)"

.PHONY: verify
verify: ## Verify the chart
	@echo "$(YELLOW)Verifying Helm chart...$(NC)"
	helm lint .
	helm template $(RELEASE_NAME) . --namespace $(NAMESPACE) > /dev/null
	@echo "$(GREEN)✓ Chart verified$(NC)"

.PHONY: test
test: ## Run Helm tests
	@echo "$(YELLOW)Running Helm tests...$(NC)"
	helm test $(RELEASE_NAME) --namespace $(NAMESPACE)
	@echo "$(GREEN)✓ Tests completed$(NC)"

.PHONY: docs
docs: ## Generate documentation
	@echo "$(YELLOW)Generating documentation...$(NC)"
	@echo "Chart: $(CHART_NAME)"
	@echo "Version: $(CHART_VERSION)"
	@echo "See README.md for full documentation"
	@echo "$(GREEN)✓ Documentation available$(NC)"

.PHONY: check-pods
check-pods: ## Check pod status
	@echo "$(YELLOW)Checking pod status...$(NC)"
	kubectl get pods -n $(NAMESPACE) -l app.kubernetes.io/name=$(CHART_NAME)

.PHONY: logs
logs: ## Show logs from all pods
	@echo "$(YELLOW)Showing logs...$(NC)"
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=$(CHART_NAME) --tail=50

.PHONY: port-forward
port-forward: ## Port-forward to metrics endpoint
	@echo "$(YELLOW)Port-forwarding to metrics endpoint...$(NC)"
	@echo "Access metrics at: http://localhost:8080/metrics"
	kubectl port-forward -n $(NAMESPACE) daemonset/$(RELEASE_NAME) 8080:8080

.PHONY: test-metrics
test-metrics: ## Test metrics endpoint (requires port-forward)
	@echo "$(YELLOW)Testing metrics endpoint...$(NC)"
	@curl -s http://localhost:8080/metrics | grep kubelet_volume_stats || echo "$(RED)No metrics found. Is port-forward running?$(NC)"

.PHONY: clean
clean: ## Clean up packaged charts
	@echo "$(YELLOW)Cleaning up...$(NC)"
	rm -f $(CHART_NAME)-*.tgz
	@echo "$(GREEN)✓ Cleanup completed$(NC)"

.PHONY: all
all: lint verify package ## Run lint, verify, and package

.DEFAULT_GOAL := help

