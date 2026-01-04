# app-deployer

> Automate the application deployment lifecycle for your projects

**app-deployer** is a custom Platform-as-a-Service (PaaS) that transforms a repository URL into a fully deployed, accessible application with automated infrastructure provisioning.

## 🎯 Vision

Input a repository URL → Output an external IP with your deployed application running on production-ready infrastructure.

## ✨ Features (Roadmap)

- **Phase 1 (MVP)** - Current
  - Deploy Node.js and Python applications from Git repositories
  - Automated container image building with Cloud Native Buildpacks
  - GCP infrastructure provisioning (VPC, GKE, Load Balancer)
  - Kubernetes deployment with Helm
  - External IP exposure

- **Phase 2** - Multi-cloud support (AWS, Azure), security scanning, observability
- **Phase 3** - Multi-tenancy, cost optimization, GitOps, self-hosting
- **Phase 4** - Edge deployments, database provisioning, marketplace

See [roadmap.md](roadmap.md) for detailed development plan.

## 🏗️ Architecture

```
Repository URL → Analyzer → Builder → Infrastructure Provisioner → Deployer → External URL
```

### Core Components

- **API Layer** - RESTful API for deployment management (Go + Fiber)
- **Orchestrator** - State machine and job queue for async processing
- **Repository Analyzer** - Language detection and requirement analysis
- **Image Builder** - Container image creation (Cloud Native Buildpacks/Nixpacks)
- **Infrastructure Provisioner** - Cloud resource creation (Pulumi)
- **Kubernetes Deployer** - Application deployment (Helm + client-go)
- **State Manager** - PostgreSQL database for tracking deployments
- **Observability** - Prometheus, Grafana, Loki for monitoring

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed technical design.

## 🚀 Quick Start

### Prerequisites

- **Go 1.21+** - [Install Go](https://go.dev/doc/install)
- **Docker & Docker Compose** - [Install Docker](https://docs.docker.com/get-docker/)
- **Make** - Build automation tool
- **GCP Account** - For deployment testing (free tier available)

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourusername/app-deployer.git
   cd app-deployer
   ```

2. **Initialize the project**
   ```bash
   make init
   ```
   This will:
   - Download Go dependencies
   - Install development tools (golangci-lint, swag)
   - Start Docker services (PostgreSQL, Redis)

3. **Start the development environment**
   ```bash
   make dev
   ```

4. **Run the API server** (in a new terminal)
   ```bash
   make run-api
   ```
   API will be available at `http://localhost:3000`

5. **Run the orchestrator worker** (in another terminal)
   ```bash
   make run-worker
   ```

### Health Check

```bash
curl http://localhost:3000/health/live
```

Expected response:
```json
{
  "status": "alive"
}
```

## 📚 Development

### Project Structure

```
app-deployer/
├── cmd/                    # Application entry points
│   ├── api/               # API server
│   ├── worker/            # Orchestrator worker
│   └── cli/               # CLI tool
├── internal/              # Private application code
│   ├── api/               # HTTP handlers, routes, middleware
│   ├── orchestrator/      # Core deployment orchestration
│   ├── analyzer/          # Repository analysis
│   ├── builder/           # Container image building
│   ├── provisioner/       # Infrastructure provisioning
│   ├── deployer/          # Kubernetes deployment
│   ├── state/             # Database models & state management
│   └── observability/     # Logging, metrics, tracing
├── pkg/                   # Public, reusable packages
├── templates/             # Infrastructure & deployment templates
├── deployments/           # Platform self-hosting manifests
└── tests/                 # Test suites
```

### Common Commands

```bash
make help              # Show all available commands
make build             # Build all binaries
make test              # Run unit tests
make test-integration  # Run integration tests
make lint              # Run linters
make fmt               # Format code
make clean             # Clean build artifacts
make tools             # Start PgAdmin and Redis Commander
```

### Running Tests

```bash
# Unit tests with coverage
make test

# View coverage report in browser
make coverage

# Integration tests (requires Docker)
make test-integration

# E2E tests (requires GCP credentials)
make test-e2e
```

### Database Access

**PostgreSQL** (default credentials for development):
- Host: `localhost:5432`
- Database: `app_deployer`
- User: `deployer`
- Password: `deployer_dev_password`

**PgAdmin** (optional, run `make tools`):
- URL: `http://localhost:5050`
- Email: `admin@app-deployer.local`
- Password: `admin`

**Redis** (default):
- Host: `localhost:6379`

**Redis Commander** (optional, run `make tools`):
- URL: `http://localhost:8081`

## 🔧 Configuration

### Environment Variables

Create a `.env` file in the project root:

```bash
# API Configuration
PORT=3000
LOG_LEVEL=info

# Database
DATABASE_URL=postgresql://deployer:deployer_dev_password@localhost:5432/app_deployer

# Redis
REDIS_URL=redis://localhost:6379

# GCP Configuration
GCP_PROJECT_ID=your-project-id
GCP_REGION=us-central1

# Container Registry
REGISTRY_TYPE=artifact-registry
REGISTRY_PROJECT=your-project-id
REGISTRY_LOCATION=us-central1
```

### Platform Configuration

See `config.yaml` for platform-wide settings (coming in Phase 1).

## 🧪 API Usage (Coming Soon)

### Create Deployment

```bash
curl -X POST http://localhost:3000/api/v1/deployments \
  -H "Content-Type: application/json" \
  -d '{
    "repo_url": "https://github.com/user/my-app",
    "cloud_provider": "gcp",
    "region": "us-central1"
  }'
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "QUEUED",
  "repo_url": "https://github.com/user/my-app",
  "created_at": "2026-01-04T12:00:00Z"
}
```

### Get Deployment Status

```bash
curl http://localhost:3000/api/v1/deployments/{id}
```

See [API Documentation](docs/api/) (coming soon) for full API reference.

## 🏗️ Engineering Principles

This project follows SRE and software engineering best practices:

- **Security**: Least privilege, secrets management, input validation, secure defaults
- **Scalability**: Stateless design, horizontal scaling, efficient resource utilization
- **Reliability**: Idempotent operations, graceful degradation, comprehensive observability
- **Code Quality**: Clean architecture, comprehensive error handling, infrastructure as code

See [CLAUDE.md](CLAUDE.md) for detailed development guidelines.

## 🌐 Cloud Provider Support

### Current (Phase 1)
- ✅ **Google Cloud Platform (GCP)** - Free tier for development

### Planned (Phase 2)
- ⏳ Amazon Web Services (AWS)
- ⏳ Microsoft Azure

The platform is designed with multi-cloud abstraction from day one.

## 📊 Current Status

**Phase 0: Foundation & Setup** - ✅ In Progress

- [x] Project initialization
- [x] Repository structure
- [x] Development environment (Docker Compose)
- [x] CI/CD pipeline (GitHub Actions)
- [ ] Initial documentation

**Next**: Phase 1 - MVP development

See [roadmap.md](roadmap.md) for complete development timeline.

## 🤝 Contributing

This is currently a development project. Contribution guidelines will be added in Phase 3.

For now, see:
- [ARCHITECTURE.md](ARCHITECTURE.md) - Technical architecture
- [roadmap.md](roadmap.md) - Development roadmap
- [CLAUDE.md](CLAUDE.md) - Development guidelines

## 📝 License

To be determined.

## 🔗 Resources

- [Architecture Documentation](ARCHITECTURE.md)
- [Development Roadmap](roadmap.md)
- [Developer Guide](CLAUDE.md)
- [API Documentation](docs/api/) (coming soon)

## 💡 Project Goals

1. **MVP in 8-12 weeks**: Deploy Node.js/Python apps to GCP from GitHub URL
2. **Production-ready in 16-20 weeks**: Multi-cloud, multi-tenant, self-hostable
3. **Cost-effective**: Leverage free tiers, optimize resource usage
4. **Extensible**: Plugin architecture for future features

---

**Status**: 🚧 Under active development - Phase 0

**Last Updated**: 2026-01-04
