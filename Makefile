.PHONY: help build test clean docker-build docker-push deploy undeploy fmt vet lint

# Variables
APP_NAME := kubelet-volume-stats-exporter
VERSION ?= latest
REGISTRY ?= docker.io/vbeaucha
IMAGE := $(REGISTRY)/$(APP_NAME):$(VERSION)
NAMESPACE := kubelet-volume-stats

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build parameters
BINARY_NAME := $(APP_NAME)
LDFLAGS := -w -s

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

build-local: ## Build the binary for local OS
	$(GOBUILD) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...

coverage: test ## Generate coverage report
	$(GOCMD) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.txt coverage.html

fmt: ## Format Go code
	$(GOFMT) ./...

vet: ## Run go vet
	$(GOVET) ./...

lint: ## Run golangci-lint
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

docker-build: ## Build Docker image
	docker build -t $(IMAGE) .
	docker tag $(IMAGE) $(REGISTRY)/$(APP_NAME):latest

docker-push: ## Push Docker image to registry
	docker push $(IMAGE)
	docker push $(REGISTRY)/$(APP_NAME):latest

docker-build-multiarch: ## Build multi-architecture Docker images
	docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE) -t $(REGISTRY)/$(APP_NAME):latest --push .

helm-install: ## Install using Helm
	helm install $(APP_NAME) ./chart -n $(NAMESPACE) --create-namespace

helm-upgrade: ## Upgrade using Helm
	helm upgrade $(APP_NAME) ./chart -n $(NAMESPACE)

helm-uninstall: ## Uninstall using Helm
	helm uninstall $(APP_NAME) -n $(NAMESPACE)

helm-lint: ## Lint Helm chart
	helm lint ./chart

helm-template: ## Template Helm chart
	helm template $(APP_NAME) ./chart -n $(NAMESPACE)

logs: ## View logs from DaemonSet pods
	kubectl logs -n $(NAMESPACE) -l app=$(APP_NAME) --tail=100 -f

status: ## Check deployment status
	@echo "=== DaemonSet Status ==="
	kubectl get daemonset -n $(NAMESPACE)
	@echo "\n=== Pods Status ==="
	kubectl get pods -n $(NAMESPACE) -o wide
	@echo "\n=== Service Status ==="
	kubectl get service -n $(NAMESPACE)

test-metrics: ## Test metrics endpoint (requires port-forward)
	@echo "Testing metrics endpoint..."
	@curl -s http://localhost:8080/metrics | grep kubelet_volume_stats || echo "No metrics found. Make sure port-forward is running: make port-forward"

port-forward: ## Port-forward to metrics endpoint
	kubectl port-forward -n $(NAMESPACE) daemonset/$(APP_NAME) 8080:8080

describe: ## Describe DaemonSet
	kubectl describe daemonset -n $(NAMESPACE) $(APP_NAME)

restart: ## Restart DaemonSet pods
	kubectl rollout restart daemonset -n $(NAMESPACE) $(APP_NAME)

all: clean deps fmt vet build docker-build ## Build everything

release: all docker-push helm-upgrade ## Build, push, and upgrade with Helm

.DEFAULT_GOAL := help

