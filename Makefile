# draino2 Makefile
# Provides common development and deployment commands

# Variables
BINARY_NAME=draino2
VERSION?=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
DOCKER_IMAGE?=nfelsen/draino2
DOCKER_TAG?=latest
KUBECONFIG?=$(HOME)/.kube/config

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_UNIX=$(BINARY_NAME)_unix

# Development tools
GOLANGCI_LINT_VERSION=v1.55.2
AIR_VERSION=v1.49.0

.PHONY: all build clean test coverage deps lint security docker-build docker-run helm-install helm-upgrade helm-uninstall k8s-deploy k8s-delete install-tools dev-watch build-all

all: clean build

# Build the binary
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/draino2

# Build for all platforms
build-all: build
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/draino2
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 ./cmd/draino2
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 ./cmd/draino2
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe ./cmd/draino2

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run tests with race detection
test-race:
	$(GOTEST) -race -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Lint code
lint:
	golangci-lint run

# Security checks
security:
	gosec ./...
	nancy sleuth

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/cosmtrek/air@$(AIR_VERSION)
	go install github.com/securecodewarrior/nancy@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Development with file watching
dev-watch:
	air

# Docker commands
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run:
	docker run -p 8080:8080 -p 9090:9090 --rm $(DOCKER_IMAGE):$(DOCKER_TAG)

# Helm commands
helm-install:
	helm install draino2 ./helm/draino2 --create-namespace --namespace draino2

helm-upgrade:
	helm upgrade draino2 ./helm/draino2 --namespace draino2

helm-uninstall:
	helm uninstall draino2 --namespace draino2

# Kubernetes deployment
k8s-deploy:
	kubectl apply -f helm/draino2/templates/

k8s-delete:
	kubectl delete -f helm/draino2/templates/

# Run locally
run:
	./$(BINARY_NAME) --config-file=config/draino2.yaml

# Run with debug logging
run-debug:
	./$(BINARY_NAME) --config-file=config/draino2.yaml --log-level=debug

# Generate manifests
manifests:
	controller-gen rbac:roleName=draino2-role crd webhook paths="./..." output:crd:artifacts:config=helm/draino2/crds

# Update dependencies
update-deps:
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Format code
fmt:
	$(GOCMD) fmt ./...

# Vet code
vet:
	$(GOCMD) vet ./...

# Check for common issues
check: fmt vet lint test

# Release preparation
release: clean build-all test-coverage security
	@echo "Release $(VERSION) prepared"

# Help
help:
	@echo "Available commands:"
	@echo "  build          - Build the binary"
	@echo "  build-all      - Build for all platforms"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  test-race      - Run tests with race detection"
	@echo "  deps           - Download dependencies"
	@echo "  lint           - Lint code"
	@echo "  security       - Run security checks"
	@echo "  install-tools  - Install development tools"
	@echo "  dev-watch      - Run with file watching"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  helm-install   - Install with Helm"
	@echo "  helm-upgrade   - Upgrade with Helm"
	@echo "  helm-uninstall - Uninstall with Helm"
	@echo "  k8s-deploy     - Deploy to Kubernetes"
	@echo "  k8s-delete     - Remove from Kubernetes"
	@echo "  run            - Run locally"
	@echo "  run-debug      - Run with debug logging"
	@echo "  check          - Format, vet, lint, and test"
	@echo "  release        - Prepare release" 