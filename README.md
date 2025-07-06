# draino2

[![Go Version](https://img.shields.io/badge/go-1.21+-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache--2.0-green)](LICENSE)

**draino2** is a modern, robust Kubernetes node draining and cordoning controller. It automates node draining based on label triggers and node conditions, with advanced features for reliability, observability, and integration.

## Features

- **Label-based Triggers**: Drain nodes when specific labels are added
- **Node Condition Monitoring**: Automatically drain nodes with problematic conditions
- **Modern Architecture**: Built with controller-runtime and Go 1.21+
- **REST API**: HTTP API for monitoring and management
- **Prometheus Metrics**: Comprehensive monitoring and alerting
- **Helm Charts**: Easy deployment to Kubernetes clusters
- **Hot Reload**: Configuration changes without restart
- **Audit Trail**: Complete logging of all operations
- **Skip Cordon Option**: Optional cordoning for different drain strategies
- **Multi-platform Support**: Linux, macOS, and Windows binaries
- **Docker Support**: Multi-stage Docker builds with security best practices
- **CI/CD Pipeline**: GitHub Actions for automated testing and releases
- **Comprehensive Testing**: Unit tests, integration tests, and security scans
- **Documentation**: Complete guides and examples

## Getting Started

### Prerequisites
- Go 1.21+
- Docker (for building images)
- kubectl (for testing)
- A Kubernetes cluster (minikube, kind, or cloud provider)

### Quick Start

1. **Clone the repository**:
   ```bash
   git clone https://github.com/nfelsen/draino2.git
   cd draino2
   ```

2. **Build the binary**:
   ```bash
   make build
   # or
   go build -o draino2 ./cmd/draino2
   ```

3. **Run locally**:
   ```bash
   ./draino2 --config-file=config/draino2.yaml
   ```

### Docker

```bash
# Build Docker image
make docker-build

# Run with Docker
make docker-run
```

### Kubernetes Deployment

#### Using Helm (Recommended)
```bash
# Install with Helm
make helm-install

# Upgrade existing installation
make helm-upgrade

# Uninstall
make helm-uninstall
```

#### Using kubectl
```bash
# Deploy to Kubernetes
make k8s-deploy

# Remove from Kubernetes
make k8s-delete
```

## Configuration

Edit `config/draino2.yaml` to customize:

- **Label Triggers**: Define which labels trigger draining
- **Exclude Labels**: Labels that prevent draining
- **Node Conditions**: Conditions that trigger automatic draining
- **Drain Settings**: Grace periods, timeouts, and behavior
- **API Settings**: REST API configuration
- **Metrics**: Prometheus metrics configuration

The configuration supports hot reload - changes are applied without restart.

### Example Configuration

```yaml
labelTriggers:
  - key: "maintenance"
    value: "true"
  - key: "decommission"
    value: "true"

excludeLabels:
  - key: "critical"
    value: "true"

nodeConditions:
  - type: "OutOfDisk"
    status: "True"
    minimumDuration: "5m"

drainSettings:
  maxGracePeriod: "8m"
  skipCordon: false
```

## Development

### Prerequisites
- Go 1.21+
- Docker
- kubectl
- A Kubernetes cluster

### Setup
```bash
# Install development tools
make install-tools

# Download dependencies
make deps

# Run tests
make test

# Run with file watching (requires air)
make dev-watch
```

### Testing
```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests with race detection
make test-race

# Run security checks
make security
```

### Building
```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build Docker image
make docker-build
```

## API Reference

### REST API Endpoints

- `GET /healthz` - Health check
- `GET /readyz` - Readiness check
- `GET /metrics` - Prometheus metrics
- `GET /api/v1/nodes` - List nodes
- `POST /api/v1/nodes/{name}/drain` - Manually drain a node
- `POST /api/v1/nodes/{name}/cordon` - Manually cordon a node

### Metrics

- `draino2_nodes_total` - Total number of nodes
- `draino2_drain_operations_total` - Total drain operations
- `draino2_drain_duration_seconds` - Drain operation duration
- `draino2_errors_total` - Total errors

## Troubleshooting

### Common Issues

1. **Permission denied**: Ensure the service account has proper RBAC permissions
2. **Config not loading**: Check the config file path and format
3. **Nodes not draining**: Verify label triggers and exclude labels
4. **API not responding**: Check if the API is enabled in config

### Logs

Enable debug logging by setting the log level:
```bash
./draino2 --config-file=config/draino2.yaml --log-level=debug
```

### Metrics

Access Prometheus metrics at `http://localhost:9090/metrics` when running locally.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

### Code Style

- Follow Go best practices
- Use `gofmt` for formatting
- Run `golint` for style checks
- Write tests for new functionality

## License

Apache 2.0. See [LICENSE](LICENSE) for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/nfelsen/draino2/issues)
- **Discussions**: [GitHub Discussions](https://github.com/nfelsen/draino2/discussions)
- **Documentation**: [Wiki](https://github.com/nfelsen/draino2/wiki)

## Acknowledgments

- Inspired by the original [Draino](https://github.com/planetlabs/draino) project
- Built with [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- Uses [Helm](https://helm.sh/) for deployment