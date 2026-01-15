# app-deployer

Automate the application deployment lifecycle for my projects.

## Overview

App-deployer is a deployment automation platform that streamlines the process of deploying applications to cloud infrastructure. It handles:

- Source code analysis
- Container image building
- Infrastructure provisioning
- Kubernetes deployment
- Service exposure and monitoring

## Project Status

### Phase 0: Project Initialization ✅
- Go module initialized
- Project structure established
- Build system (Makefile) configured
- Core dependencies installed

### Phase 1: Core Database Connectivity ✅
- Database connection layer implemented
- Connection pooling configured
- Health check functionality added
- State management models defined
- Repository pattern implemented

### Phase 2: HTTP API Server ✅
- RESTful API endpoints for deployments, infrastructure, and builds
- Request/response models with JSON serialization
- HTTP middleware (logging, CORS, error handling, recovery)
- Chi router for clean routing
- Health check endpoint
- Graceful server shutdown

### Phase 3: Source Code Analysis ✅
- Multi-language detection (Go, Node.js, Python, Java, Rust, Ruby, PHP, .NET)
- Framework detection (Express, Flask, Django, Spring Boot, Gin, etc.)
- Dependency parsing (package.json, go.mod, requirements.txt, pom.xml, etc.)
- Build tool detection (npm, yarn, go, pip, maven, gradle)
- Runtime version detection
- Port and command inference
- File upload support for analysis
- Confidence scoring

## Prerequisites

- Go 1.25.5 or later
- PostgreSQL 12 or later (for production)
- Make

## Installation

1. Clone the repository:
```bash
git clone https://github.com/mateus/app-deployer.git
cd app-deployer
```

2. Install dependencies:
```bash
make deps
```

3. Configure the application:
   - Copy `config.yaml` and adjust settings
   - Or use environment variables (see Configuration section)

## Configuration

The application can be configured via `config.yaml` or environment variables.

### Database Configuration

```yaml
database:
  host: localhost
  port: 5432
  user: deployer
  password: deployer_dev_password
  dbname: app_deployer
  sslmode: disable
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m
```

### Server Configuration

```yaml
server:
  port: "3000"
  read_timeout: 10s
  write_timeout: 10s
  log_level: info
```

## Building

Build the application:

```bash
make build
```

The binary will be created at `bin/app-deployer`.

## Running

Start the application:

```bash
make run
```

Or run the built binary:

```bash
./bin/app-deployer
```

## Testing

Run tests:

```bash
make test
```

Run tests with coverage:

```bash
make test-coverage
```

## Database Initialization

Initialize the database schema:

```bash
make init-db
```

This will create the necessary tables:
- `deployments` - Deployment records
- `infrastructures` - Infrastructure state
- `builds` - Container build history

## Project Structure

```
app-deployer/
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── api/            # HTTP API handlers
│   ├── analyzer/       # Source code analysis
│   ├── builder/        # Container image building
│   ├── deployer/       # Deployment orchestration
│   ├── orchestrator/   # Workflow orchestration
│   ├── provisioner/    # Infrastructure provisioning
│   ├── state/          # State management and repository
│   └── observability/  # Logging and monitoring
├── pkg/
│   ├── config/         # Configuration management
│   ├── database/       # Database connection and utilities
│   └── utils/          # Shared utilities
├── scripts/
│   └── setup/          # Setup and initialization scripts
├── config.yaml         # Application configuration
├── Makefile           # Build automation
└── README.md          # This file
```

## Data Models

### Deployment

Represents a deployment instance with metadata about the application being deployed.

Fields:
- `ID` - Unique identifier (UUID)
- `Name` - Deployment name
- `AppName` - Application name
- `Version` - Application version
- `Status` - Deployment status (PENDING, BUILDING, PROVISIONING, DEPLOYING, EXPOSED, FAILED)
- `Cloud` - Target cloud provider (gcp, aws, azure)
- `Region` - Cloud region
- `ExternalIP` - Exposed IP address
- `ExternalURL` - Exposed URL
- Timestamps: `CreatedAt`, `UpdatedAt`, `DeployedAt`

### Infrastructure

Tracks provisioned infrastructure for deployments.

Fields:
- `ID` - Unique identifier (UUID)
- `DeploymentID` - Associated deployment
- `ClusterName` - Kubernetes cluster name
- `Namespace` - Kubernetes namespace
- `ServiceName` - Kubernetes service name
- `Status` - Infrastructure status (PROVISIONING, READY, FAILED)
- `Config` - Infrastructure configuration (JSONB)

### Build

Records container image builds.

Fields:
- `ID` - Unique identifier (UUID)
- `DeploymentID` - Associated deployment
- `ImageTag` - Container image tag
- `Status` - Build status (PENDING, BUILDING, SUCCESS, FAILED)
- `BuildLog` - Build output logs
- `StartedAt`, `CompletedAt` - Build timing

## Development

### Make Targets

- `make build` - Build the application
- `make test` - Run tests
- `make test-race` - Run tests with race detection (requires CGO)
- `make test-coverage` - Generate coverage report
- `make clean` - Clean build artifacts
- `make run` - Run the application
- `make init-db` - Initialize database
- `make fmt` - Format code
- `make lint` - Run linters
- `make deps` - Install dependencies

## API Documentation

See [docs/API.md](docs/API.md) for complete API documentation with examples.

### Quick API Examples

```bash
# Check health
curl http://localhost:3000/health

# Create a deployment
curl -X POST http://localhost:3000/api/v1/deployments \
  -H "Content-Type: application/json" \
  -d '{"name":"test","app_name":"myapp","version":"v1.0.0"}'

# List deployments
curl http://localhost:3000/api/v1/deployments
```

## Roadmap

- [x] Phase 0: Project initialization
- [x] Phase 1: Core database connectivity
- [x] Phase 2: HTTP API server
- [x] Phase 3: Source code analysis
- [ ] Phase 4: Container image building
- [ ] Phase 5: Infrastructure provisioning
- [ ] Phase 6: Deployment orchestration
- [ ] Phase 7: Service exposure
- [ ] Phase 8: Observability

## License

MIT
