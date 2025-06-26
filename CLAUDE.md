# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoPool is a high-performance Bitcoin mining pool implementation written in Go 1.24+. This is currently a documentation-only project in the planning/design phase - no Go source code has been implemented yet.

## Architecture

The system is designed as a microservices architecture with the following core services:

- **stratumd** (`/cmd/stratumd/`) - Stratum gateway managing thousands of concurrent TCP connections
- **jobmanager** (`/cmd/jobmanager/`) - Interfaces with Bitcoin Core, creates mining jobs
- **shareproc** (`/cmd/shareproc/`) - Core validation engine for share processing
- **blocksubmit** (`/cmd/blocksubmit/`) - Minimal service for block submission to Bitcoin Core
- **statsd** (`/cmd/statsd/`) - Statistics pipeline writing to InfluxDB
- **payoutd** (`/cmd/payoutd/`) - Financial ledger handling PPLNS payouts
- **apiserver** (`/cmd/apiserver/`) - REST API service

## Planned Repository Structure

The project will follow Go Standard Project Layout:

```
/cmd/           # Main applications for each microservice
/internal/      # Private application library code
/pkg/           # Public library code
/proto/         # Protobuf definitions
/api/           # OpenAPI/Swagger specs
/configs/       # Configuration file templates
/deployments/   # Docker/deployment files
```

## Performance Requirements

This is a **performance-first** project with these non-negotiable requirements:

- Hot path optimizations (stratumd → shareproc) must minimize heap allocations
- Use `sync.Pool` for object reuse in critical paths
- Leverage Go's concurrency model with goroutines and channels
- Profile regularly with `pprof` to identify bottlenecks
- Optimize for low-latency block submission (blocksubmit service)

## Communication & Data Flow

- **Inter-service**: Apache Kafka with Protocol Buffers for message serialization
- **Databases**: PostgreSQL (transactional), InfluxDB (time-series), Redis (cache/state)
- **Hot vs Warm Paths**:
  - Hot Path: Block-winning shares (ultra-low latency)
  - Warm Path: Regular shares for accounting (high throughput)

## Key Implementation Principles

- Use standard library wherever possible ("A little copying is better than a little dependency")
- Explicit error handling with error values (no panics for recoverable errors)
- Structured logging with `log/slog` and JSON output
- Configuration via environment variables and flags (12-Factor App)
- Context-based cancellation and `errgroup` for goroutine lifecycle management

## Protocol Implementations

- **Stratum V1**: JSON-RPC over TCP for miner connections (see docs/stratum-v1.md)
- **PPLNS**: Pay-Per-Last-N-Shares reward system (see docs/pplns.md)
- **Bitcoin Core Integration**: ZMQ notifications and RPC calls

## Code Standards

### Structured Logging

Use `log/slog` with consistent levels:

```go
logger.Info("starting service", "component", "stratifier", "port", 3333)
logger.Error("connection failed", "error", err, "host", host)
```

### Error Handling

Always wrap errors with context:

```go
if err := client.Connect(); err != nil {
    return fmt.Errorf("failed to connect to bitcoind: %w", err)
}
```

Never ignore errors - always check return values.

### Testing

You MUST follow TDD (test driven development) and ensure comprehensive tests exist for all critical logic. Aim for at least 80% code coverage.

Use table-driven tests:

```go
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {"valid config", `{"mindiff": 1}`, &Config{MinDiff: 1}, false},
        {"invalid json", `{invalid}`, nil, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
            }
            // ... assertions
        })
    }
}
```

## Commit Messages

Use Conventional Commits format:

```text
feat(stratifier): implement vardiff algorithm
fix(connector): handle partial TCP writes
test(config): add validation test cases
```

## Development Patterns

- Keep interfaces small and focused
- Use explicit returns, avoid named returns
- Document public APIs with godoc comments

## Development Commands

### Setup
```bash
# Install development tools (golangci-lint, etc.)
make install-tools

# Initialize Go modules (when first Go file is added)
go mod init github.com/bardlex/gopool
```

### Building & Testing
```bash
# Build all services
make build

# Run tests with coverage
make test

# Run quick tests only
make test-short
```

### Code Quality
```bash
# Run all linting checks
make lint

# Run linting with auto-fix
make lint-fix

# Format code
make fmt

# Run full CI pipeline (format, lint, test)
make ci

# Run all quality checks
make check
```

### Protobuf Generation
```bash
# Generate protobuf files (when proto files exist)
protoc --go_out=. proto/**/*.proto
```

## Documentation

Comprehensive technical specifications are available in `/docs/`:
- `architecture.md` - Complete system architecture and design
- `pplns.md` - PPLNS algorithm and economic model
- `stratum-v1.md` - Stratum protocol implementation details
