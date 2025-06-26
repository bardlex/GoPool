# GOMP (Go Mining Pool)

A high-performance Bitcoin mining pool implementation written in Go 1.24+, designed for scalability, reliability, and optimal performance.

## 🏗️ Architecture

GOMP follows a microservices architecture with the following core services:

- **stratumd** - Stratum gateway managing thousands of concurrent TCP connections
- **jobmanager** - Interfaces with Bitcoin Core, creates mining jobs  
- **shareproc** - Core validation engine for share processing
- **blocksubmit** - Minimal service for block submission to Bitcoin Core
- **statsd** - Statistics pipeline writing to InfluxDB
- **payoutd** - Financial ledger handling PPLNS payouts
- **apiserver** - REST API service

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Bitcoin Core (with RPC enabled)
- Apache Kafka
- PostgreSQL
- Redis
- InfluxDB

### Building

```bash
# Install development tools
make install-tools

# Build all services
make build

# Run tests
make test

# Run linting
make lint
```

### Running Services

Each service can be configured via environment variables:

```bash
# Start stratumd (Stratum server)
./bin/stratumd

# Start jobmanager (Job creation)
./bin/jobmanager

# Start shareproc (Share validation)
./bin/shareproc

# Start blocksubmit (Block submission)
./bin/blocksubmit
```

## ⚙️ Configuration

Services are configured via environment variables:

### Core Settings
- `SERVICE_NAME` - Service identifier (default: "gomp")
- `VERSION` - Service version (default: "dev")
- `ENVIRONMENT` - Environment (default: "development")
- `LOG_LEVEL` - Logging level (default: "info")
- `LOG_FORMAT` - Log format: "json" or "text" (default: "json")

### Network
- `LISTEN_ADDR` - Listen address (default: "0.0.0.0")
- `LISTEN_PORT` - Listen port (default: 3333)

### Bitcoin Core
- `BITCOIN_RPC_HOST` - Bitcoin Core RPC host (default: "localhost")
- `BITCOIN_RPC_PORT` - Bitcoin Core RPC port (default: 8332)
- `BITCOIN_RPC_USER` - Bitcoin Core RPC username
- `BITCOIN_RPC_PASSWORD` - Bitcoin Core RPC password
- `BITCOIN_ZMQ_ADDR` - Bitcoin Core ZMQ address (default: "tcp://localhost:28332")

### Pool Settings
- `POOL_FEE_PERCENT` - Pool fee percentage (default: 1.0)
- `PPLNS_WINDOW_FACTOR` - PPLNS window multiplier (default: 2.0)
- `MIN_DIFFICULTY` - Minimum share difficulty (default: 1.0)
- `MAX_DIFFICULTY` - Maximum share difficulty (default: 1000000.0)

### Performance
- `MAX_CONNECTIONS` - Maximum concurrent connections (default: 10000)
- `READ_TIMEOUT` - Connection read timeout (default: "30s")
- `WRITE_TIMEOUT` - Connection write timeout (default: "30s")
- `WORKER_POOL_SIZE` - Worker pool size (default: 100)

## 🏛️ Architecture Details

### Data Flow

1. **Job Creation**: `jobmanager` monitors Bitcoin Core for new blocks and creates mining jobs
2. **Job Distribution**: Jobs are published to Kafka and consumed by `stratumd` instances
3. **Share Submission**: Miners submit shares via Stratum protocol to `stratumd`
4. **Share Processing**: `shareproc` validates shares and identifies block candidates
5. **Block Submission**: `blocksubmit` immediately submits solved blocks to Bitcoin Core
6. **Statistics**: `statsd` processes share data for metrics and analytics
7. **Payouts**: `payoutd` calculates PPLNS rewards and manages balances

### Hot Path vs Warm Path

- **Hot Path**: Block-winning shares (ultra-low latency)
  - `stratumd` → `shareproc` → `blocksubmit` → Bitcoin Core
- **Warm Path**: Regular shares for accounting (high throughput)  
  - `stratumd` → `shareproc` → `statsd`/`payoutd`

### Performance Optimizations

- **Zero-allocation hot paths** using `sync.Pool` for object reuse
- **Concurrent I/O** with goroutines and channels
- **Efficient networking** with proper TCP tuning
- **Structured logging** with `log/slog`
- **Comprehensive profiling** with `pprof`

## 📊 Monitoring

GOMP provides comprehensive observability:

- **Structured JSON logs** with contextual information
- **Performance metrics** via duration logging
- **Connection tracking** with session management
- **Share statistics** with validation status
- **Block submission latency** monitoring

## 🧪 Testing

```bash
# Run all tests with coverage
make test

# Run only short tests
make test-short

# Run linting
make lint

# Run full CI pipeline
make ci
```

## 🔧 Development

### Project Structure

```
/cmd/           # Main applications for each microservice
/internal/      # Private application library code
/pkg/           # Public library code  
/proto/         # Protobuf definitions
/docs/          # Technical documentation
/configs/       # Configuration templates
```

### Adding New Features

1. Follow TDD (Test-Driven Development)
2. Maintain >80% code coverage
3. Use table-driven tests
4. Follow Go idioms and best practices
5. Add comprehensive logging
6. Profile performance-critical code

### Code Standards

- **Error Handling**: Always wrap errors with context
- **Logging**: Use structured logging with `log/slog`
- **Concurrency**: Use `context` for cancellation and `errgroup` for lifecycle management
- **Performance**: Profile with `pprof` and optimize hot paths
- **Testing**: Write comprehensive tests with good coverage

## 📚 Documentation

Detailed technical documentation is available in the `/docs/` directory:

- [`docs/architecture.md`](docs/architecture.md) - Complete system architecture
- [`docs/stratum-v1.md`](docs/stratum-v1.md) - Stratum protocol implementation
- [`docs/pplns.md`](docs/pplns.md) - PPLNS algorithm and economics

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for your changes
4. Ensure all tests pass and linting is clean
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🚨 Security

This is a financial system handling Bitcoin transactions. Security considerations:

- **Never commit secrets** to the repository
- **Validate all inputs** from external sources
- **Use secure connections** for production deployments
- **Regular security audits** of critical code paths
- **Proper key management** for Bitcoin addresses

## 🎯 Roadmap

- [ ] Kafka integration for inter-service communication
- [ ] Complete protobuf implementation
- [ ] Database schema and migrations
- [ ] Web dashboard and API
- [ ] Docker deployment configurations
- [ ] Kubernetes manifests
- [ ] Comprehensive monitoring and alerting
- [ ] Load testing and benchmarks

---

**Status**: 🚧 Under Development

This is a greenfield implementation designed for production use. The core architecture and services are implemented, with ongoing work on integration and deployment.