# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository automates the application deployment lifecycle. The codebase is currently in its initial stages.

## Engineering Principles

This project follows SRE and software engineering best practices with focus on:

**Security**
- Least privilege access patterns
- Secrets management (never hardcoded credentials)
- Input validation and sanitization
- Secure defaults

**Scalability**
- Stateless designs where possible
- Efficient resource utilization
- Horizontal scaling considerations
- Async/concurrent operations for I/O-bound tasks

**Reliability**
- Graceful degradation and failure handling
- Idempotent operations (safe to retry)
- Comprehensive logging and observability
- Health checks and monitoring hooks
- Rollback capabilities

**Code Quality**
- Clean, maintainable architecture
- Comprehensive error handling
- Infrastructure as code principles
- Configuration separated from code

## Architecture

**app-deployer** is a custom PaaS platform built on open-source foundations. See `ARCHITECTURE.md` for detailed component breakdown.

### Core Flow
```
Repository URL → Analyzer → Builder → Infrastructure Provisioner → Deployer → External URL
```

### Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Language** | Go | API, orchestrator, core logic |
| **API Framework** | Fiber/Echo | RESTful API layer |
| **Database** | PostgreSQL | State management, deployment tracking |
| **Queue** | Redis | Async job processing |
| **Builder** | Cloud Native Buildpacks / Nixpacks | Container image building |
| **IaC** | Pulumi Automation API | Infrastructure provisioning |
| **Orchestration** | Kubernetes + Helm | Application deployment |
| **Registry** | Harbor | Container image storage |
| **Observability** | Prometheus + Grafana + Loki | Metrics, dashboards, logs |

### Repository Structure
```
app-deployer/
├── cmd/
│   ├── api/           # API server entry point
│   ├── worker/        # Orchestrator worker entry point
│   └── cli/           # CLI tool
├── internal/
│   ├── api/           # HTTP handlers, routes, middleware
│   ├── orchestrator/  # Core deployment orchestration
│   ├── analyzer/      # Repository analysis & language detection
│   ├── builder/       # Container image building
│   ├── provisioner/   # Infrastructure provisioning (Pulumi)
│   ├── deployer/      # Kubernetes deployment (Helm)
│   ├── state/         # Database models & state management
│   └── observability/ # Logging, metrics, tracing
├── pkg/
│   ├── models/        # Shared data models
│   ├── utils/         # Shared utilities
│   └── config/        # Configuration loading
├── templates/
│   ├── pulumi/        # Infrastructure templates (AWS, GCP, Azure)
│   └── helm/          # Helm chart templates
├── deployments/
│   └── kubernetes/    # Platform self-hosting manifests
└── tests/
    ├── unit/
    ├── integration/
    └── e2e/
```

## Development Commands

### Local Development
```bash
make dev              # Start local development stack (docker-compose)
make build            # Build all binaries
make test             # Run unit tests
make test-integration # Run integration tests
make lint             # Run linters (golangci-lint)
make migrate          # Run database migrations
```

### Running Components
```bash
# API Server
go run cmd/api/main.go

# Orchestrator Worker
go run cmd/worker/main.go

# CLI
go run cmd/cli/main.go deploy https://github.com/user/repo
```

### Testing
```bash
# Unit tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# Specific package
go test -v ./internal/analyzer/...

# Integration tests (requires Docker)
go test -v -tags=integration ./tests/integration/...

# E2E tests (requires cloud credentials)
go test -v -tags=e2e ./tests/e2e/...
```

## Development Workflow

### Adding a New Feature

1. **Reference the roadmap** - Check `roadmap.md` for current phase and priorities
2. **Design first** - For complex features, create an ADR (Architecture Decision Record) in `docs/adr/`
3. **Write tests first** - TDD approach preferred
4. **Implement** - Follow SRE principles (security, scalability, reliability)
5. **Update docs** - Update relevant documentation
6. **Create PR** - Include tests, update CHANGELOG.md

### Code Organization

- **cmd/** - Entry points only, minimal logic
- **internal/** - Private application code, organized by domain
- **pkg/** - Public, reusable packages (if any)
- **templates/** - Infrastructure and deployment templates
- **tests/** - All test code

### Key Architectural Patterns

**Idempotency**: All operations must be idempotent (safe to retry)
- Use unique deployment IDs for deduplication
- Check current state before applying changes
- Infrastructure operations use Pulumi's declarative model

**State Machine**: Deployments follow a strict state progression
```
QUEUED → ANALYZING → BUILDING → PROVISIONING → DEPLOYING → HEALTHY → EXPOSED
                                         ↓
                                      FAILED → ROLLING_BACK
```

**Async Processing**: Long-running operations use job queue
- API immediately returns deployment ID
- Worker processes jobs asynchronously
- Logs streamed to database for retrieval

**Error Handling**: Fail fast, recover gracefully
- Validate inputs early
- Retry with exponential backoff
- Rollback on failure
- Log all errors with context

## Configuration

### Platform Configuration (`config.yaml`)
```yaml
platform:
  default_cloud: gcp      # gcp, aws, azure
  default_region: us-central1

database:
  host: localhost
  port: 5432
  name: app_deployer

registry:
  type: artifact-registry  # artifact-registry, harbor, ecr, acr
  project: your-gcp-project-id
  location: us-central1

limits:
  max_deployments_per_user: 10
  max_build_time: 30m
```

### Deployment Configuration (`deployer.yaml` - optional per-deployment)
```yaml
deployment:
  cloud_provider: gcp
  region: us-central1
  application:
    port: 3000
    replicas: 3
    resources:
      cpu: 1000m
      memory: 1Gi
```

## Testing Strategy

### Test Pyramid
- **Unit tests** (80%): Fast, isolated, mock external dependencies
- **Integration tests** (15%): Component interactions, real dependencies in Docker
- **E2E tests** (5%): Full deployment flow, real cloud resources

### Integration Test Requirements
- Docker for running PostgreSQL, Redis, Harbor
- Local Kubernetes (kind, k3s) for deployment testing
- Mock cloud provider APIs (Localstack for AWS) where possible

### E2E Test Requirements
- Real cloud provider accounts (separate test accounts)
- Automated cleanup after tests
- Tagged resources for cost tracking
- Run in CI on main branch only (expensive)

## Cloud Provider Support

### Phase 1 (MVP): GCP Only
- GKE for Kubernetes (e2-small nodes, free tier eligible)
- VPC, subnets, firewall rules
- GCS for Pulumi state
- Artifact Registry for container images (0.5 GB free/month)
- External IP for service exposure (DNS service to be added later)

### Phase 2: Multi-Cloud
- **AWS**: EKS, VPC, S3, ECR
- **Azure**: AKS, VNet, Blob Storage, ACR

### Provider Abstraction
All cloud-specific code lives in `internal/provisioner/{gcp,aws,azure}/`
Common interface defined in `internal/provisioner/interface.go`
**Note**: MVP starts with GCP, designed for easy multi-cloud expansion

## Security Guidelines

### Secrets Management
- **Never** commit secrets to version control
- Use cloud provider secret managers (AWS Secrets Manager, GCP Secret Manager, Azure Key Vault)
- Inject secrets at runtime via Kubernetes external-secrets operator
- Rotate credentials regularly

### Container Security
- Non-root container users
- Read-only root filesystems
- Minimal base images
- Vulnerability scanning with Trivy before deployment
- Image signing with Sigstore/cosign

### Infrastructure Security
- Least privilege IAM roles
- Security groups with minimal port exposure
- Private subnets for workloads
- Encryption at rest and in transit
- Network policies in Kubernetes

## Observability

### Logging
- Structured logging with zerolog or zap
- Log levels: debug, info, warn, error
- Include context: deployment_id, user_id, request_id
- Aggregate logs with Loki or ELK

### Metrics (Prometheus)
- Deployment counters (success, failure)
- Build time histogram
- Provisioning time histogram
- Active deployments gauge
- API request latency

### Tracing
- Distributed tracing with OpenTelemetry
- Trace full deployment lifecycle
- Identify bottlenecks and failures

### Health Checks
- `/health/live` - Liveness (is process running?)
- `/health/ready` - Readiness (can accept traffic?)
- `/metrics` - Prometheus metrics endpoint

## Current Phase

**Phase 0: Foundation & Setup**
- Setting up development environment
- Establishing project structure
- Configuring CI/CD pipeline

See `roadmap.md` for detailed phase breakdown and task list.

## Common Pitfalls to Avoid

- **Hardcoded credentials** - Always use secret managers
- **Blocking operations in API** - Use async job queue for long-running tasks
- **Missing error context** - Always wrap errors with additional context
- **No timeout handling** - Set timeouts for all external operations
- **Ignoring rollback** - Always provide a way to undo changes
- **Poor resource cleanup** - Ensure all cloud resources are properly destroyed
- **Missing cost tags** - Tag all cloud resources for cost tracking

## Additional Resources

- Full architecture: `ARCHITECTURE.md`
- Development roadmap: `roadmap.md`
- API documentation: `/docs/api/` (when available)
- ADRs: `/docs/adr/` (when available)
