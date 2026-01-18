# Development Roadmap

## Overview

This roadmap breaks down the development of **app-deployer** into manageable phases, from MVP to production-ready platform. Each phase builds upon the previous, allowing for incremental testing and validation.

**Estimated Timeline**: MVP in 8-12 weeks, Production-ready in 16-20 weeks

---

## Phase 0: Foundation & Setup (Week 1-2)

### Goals
- Set up development environment
- Initialize project structure
- Establish CI/CD pipeline
- Set up local testing infrastructure

### Tasks

#### Project Initialization
- [x] Initialize Go module (`go mod init github.com/yourusername/app-deployer`)
- [x] Create repository structure (see ARCHITECTURE.md)
- [x] Set up Git hooks (pre-commit for linting, formatting)
- [x] Configure editorconfig and linting rules (golangci-lint)

#### Development Environment
- [x] Create docker-compose.yml for local development
  - PostgreSQL database
  - Redis for job queue
  - Harbor registry (or use Docker Hub for MVP)
  - Localstack for AWS emulation (optional)
- [x] Write Makefile with common commands
  - `make build` - Build all binaries
  - `make test` - Run tests
  - `make dev` - Start local development environment
  - `make lint` - Run linters
  - `make migrate` - Run database migrations

#### CI/CD Setup
- [x] Create GitHub Actions workflows
  - `.github/workflows/ci.yml` - Unified CI (lint, test, build) on all branches
  - `.github/workflows/build.yml` - Multi-platform builds and Docker images
- [ ] Set up branch protection rules
- [ ] Configure dependabot for dependency updates

#### Documentation
- [x] Update README.md with project overview and setup instructions
- [ ] Create CONTRIBUTING.md with development guidelines
- [ ] Set up documentation site structure (MkDocs or similar)

**Deliverables**:
- Runnable local development environment
- Automated testing pipeline
- Project skeleton with proper structure

---

## Phase 1: MVP - Single Cloud, Happy Path (Week 3-6)

### Goals
- Deploy a simple Node.js/Python app to GCP
- Prove end-to-end flow works
- Single-tenant, basic security
- Manual testing only
- Return external IP (DNS service to be added later)

### Tasks

#### 1.1 Database & State Management (Week 3)
- [x] Design database schema (PostgreSQL)
- [x] Set up database migrations (golang-migrate or GORM auto-migrate)
- [x] Implement state manager package
  - Deployment CRUD operations
  - Infrastructure state tracking
  - Build history
- [x] Write unit tests for state manager
- [x] Create database seeding for local development

**Files**: `internal/state/models.go`, `internal/state/repository.go`

#### 1.2 API Layer (Week 3)
- [x] Set up Fiber/Echo HTTP framework
- [x] Implement core endpoints
  - `POST /deployments` - Create deployment
  - `GET /deployments/:id` - Get deployment status
  - `GET /deployments/:id/logs` - Stream logs (basic polling, not WebSocket yet)
  - `DELETE /deployments/:id` - Delete deployment
- [x] Add basic authentication (API key in header)
- [x] Request validation middleware
- [x] Error handling middleware
- [ ] OpenAPI/Swagger documentation generation
- [ ] Write API integration tests

**Files**: `cmd/api/main.go`, `internal/api/handlers/`, `internal/api/middleware/`

#### 1.3 Repository Analyzer (Week 4)
- [x] Implement Git clone functionality (go-git library)
- [x] Create language detection logic
  - Node.js (package.json)
  - Python (requirements.txt, pyproject.toml)
  - Go (go.mod)
- [x] Parse package files to extract metadata
- [x] Determine default ports by framework
- [x] Estimate resource requirements (basic heuristics)
- [x] Write unit tests with fixture repositories
- [ ] Integration test with real GitHub repos

**Files**: `internal/analyzer/analyzer.go`, `internal/analyzer/detectors/`

#### 1.4 Image Builder (Week 4-5)
- [x] Integrate Cloud Native Buildpacks (pack CLI)
- [x] Implement buildpack builder selection logic
- [x] Add fallback to Dockerfile if present
- [x] Implement image tagging strategy (git-sha, latest)
- [x] Push images to GCP Artifact Registry (free tier) or Harbor
- [x] Capture and store build logs
- [x] Handle build failures gracefully
- [ ] Write integration tests with sample apps
- [ ] Add build timeout handling (30 min max)

**Files**: `internal/builder/buildpack.go`, `internal/builder/docker.go`

**Note**: For MVP, shell out to `pack` CLI. Phase 2 can use native libraries.
**Registry**: GCP Artifact Registry has 0.5 GB free storage/month, sufficient for MVP

#### 1.5 Infrastructure Provisioner - GCP Only (Week 5)
- [x] Set up Pulumi project structure
- [x] Implement GCP VPC creation
  - Public and private subnets
  - Cloud Router
  - Cloud NAT
  - Firewall rules
- [x] Implement GKE cluster provisioning
  - Node pool configuration (e2-small, 2 nodes)
  - Service account with minimal permissions
  - Cluster authentication
  - Autopilot or Standard mode (Standard for cost control)
- [x] Implement firewall rules (minimal ports)
- [x] Save Pulumi state to GCS backend
- [x] Implement infrastructure destroy logic
- [ ] Write integration tests (creates and destroys real resources)
- [x] Add labels for all resources (cost tracking, environment)

**Files**: `internal/provisioner/pulumi.go`, `internal/provisioner/gcp/`

**Cost Control**: Use e2-small nodes (free tier eligible), 2 node minimum, preemptible VMs option

#### 1.6 Kubernetes Deployer (Week 6)
- [x] Create Helm chart template generator
- [x] Implement Helm chart values population from analyzer output
- [x] Deploy Helm chart to Kubernetes using client-go
- [x] Implement pod health checking (wait for Ready status)
- [x] Create LoadBalancer service
- [x] Retrieve external IP from LoadBalancer
- [ ] Write integration tests with local kind cluster
- [x] Add deployment timeout handling (10 min max)

**Files**: `internal/deployer/helm.go`, `templates/helm/base-app/`

#### 1.7 Orchestrator (Week 6)
- [x] Implement deployment state machine
- [x] Create job queue with Redis
- [x] Implement worker process for async jobs
- [x] Coordinate analyzer → builder → provisioner → deployer flow
- [x] Implement error handling and rollback logic
- [x] Add retry logic with exponential backoff
- [x] Stream logs to database for retrieval
- [x] Write end-to-end integration tests

**Files**: `internal/orchestrator/engine.go`, `internal/orchestrator/worker.go`, `tests/e2e/`

#### 1.8 MVP Testing & Validation
- [ ] Deploy sample Node.js Express app end-to-end
- [ ] Deploy sample Python Flask app end-to-end
- [ ] Verify external URL accessibility
- [ ] Test deployment deletion and cleanup
- [ ] Load testing (10 concurrent deployments)
- [ ] Document known issues and limitations

**Deliverables**:
- Working MVP that deploys apps to GCP GKE
- Basic API for deployment management
- Returns external IP for deployed applications
- End-to-end tests covering happy path
- Multi-cloud foundation for future AWS/Azure support

---

## Phase 2: Multi-Cloud & Robustness (Week 7-10)

### Goals
- Add GCP and Azure support
- Improve error handling and observability
- Add vulnerability scanning
- Implement proper rollback mechanism

### Tasks

#### 2.1 Multi-Cloud Support (Week 7-8)
- [x] Refactor provisioner for cloud provider abstraction (GCP implementation complete)
- [ ] Implement AWS provisioner
  - VPC creation
  - EKS cluster
  - Application Load Balancer
  - State in S3
- [ ] Implement Azure provisioner
  - VNet creation
  - AKS cluster
  - Azure Load Balancer
  - State in Azure Blob
- [ ] Add cloud provider selection to API
- [ ] Update configuration schema for multi-cloud
- [ ] Write integration tests for each cloud provider

**Files**: `internal/provisioner/interface.go`, `internal/provisioner/gcp/`, `internal/provisioner/azure/`

#### 2.2 Security Enhancements (Week 8)
- [x] Integrate Trivy for vulnerability scanning
- [x] Scan images before deployment
- [x] Fail deployment on critical vulnerabilities (configurable)
- [ ] Implement secrets management
  - Integration with AWS Secrets Manager
  - Integration with GCP Secret Manager
  - Integration with Azure Key Vault
- [ ] Add external-secrets operator to Kubernetes deployments
- [x] Implement API authentication with JWT
- [x] Add rate limiting to API endpoints
- [ ] Security audit of all components

**Files**: `internal/builder/scanner/trivy.go`, `internal/api/auth.go`, `internal/api/ratelimit.go`

#### 2.3 Observability (Week 9)
- [x] Implement structured logging (zerolog or zap)
- [x] Add Prometheus metrics
  - Deployment counters (success/failure)
  - Build time histogram
  - Provisioning time histogram
  - Active deployments gauge
  - HTTP request metrics
  - Queue metrics
  - Vulnerability scan metrics
- [x] Create Grafana dashboards
- [x] Implement distributed tracing with OpenTelemetry
  - OTLP HTTP exporter to Jaeger/collector
  - HTTP request tracing middleware
  - Custom span attributes for deployments, builds, infrastructure
  - Context propagation for distributed traces
- [x] Add health check endpoints
  - `/health/live` - Liveness probe
  - `/health/ready` - Readiness probe
  - `/metrics` - Prometheus metrics
- [ ] Set up log aggregation (Loki or ELK)

**Files**: `internal/observability/metrics.go`, `internal/observability/tracing.go`, `internal/api/metrics_middleware.go`, `internal/api/tracing_middleware.go`, `deployments/monitoring/`

#### 2.4 Rollback & Recovery (Week 9-10)
- [ ] Store previous deployment versions in state
- [ ] Implement rollback endpoint
- [ ] Test rollback with Helm rollback
- [ ] Implement blue-green deployment option
- [ ] Add deployment history tracking
- [ ] Implement automated health check rollback
- [ ] Test disaster recovery procedures
- [ ] Document rollback procedures

**Files**: `internal/deployer/rollback.go`

#### 2.5 Advanced Features (Week 10)
- [ ] Implement auto-scaling configuration
  - HorizontalPodAutoscaler
  - Cluster autoscaler
- [ ] Add custom domain support
- [ ] Integrate cert-manager for SSL certificates
- [ ] Implement DNS management
  - Route53 for AWS
  - Cloud DNS for GCP
  - Azure DNS for Azure
- [ ] Add deployment environment support (dev, staging, prod)
- [ ] Implement resource quotas per deployment

**Files**: `internal/deployer/autoscaling.go`, `internal/dns/`

**Deliverables**:
- Multi-cloud deployment support (AWS, GCP, Azure)
- Security scanning and secrets management
- Comprehensive observability
- Reliable rollback mechanism

---

## Phase 3: Production Readiness (Week 11-14)

### Goals
- Multi-tenancy and RBAC
- Cost optimization
- Advanced deployment strategies
- Self-hosting capability

### Tasks

#### 3.1 Multi-Tenancy (Week 11)
- [ ] Implement user/organization model
- [ ] Add RBAC (role-based access control)
  - Admin, Developer, Viewer roles
- [ ] Namespace isolation per user/org
- [ ] Resource quotas per user/org
- [ ] Cost tracking per user/org
- [ ] Billing integration (Stripe)
- [ ] Admin dashboard for platform management

**Files**: `internal/auth/`, `internal/rbac/`, `internal/billing/`

#### 3.2 Cost Optimization (Week 11-12)
- [ ] Implement cluster sharing (multiple apps per cluster)
- [ ] Add node auto-scaling
- [ ] Implement idle deployment shutdown
- [ ] Add spot instance support for non-prod
- [ ] Cost estimation before deployment
- [ ] Cost alerts and budgets
- [ ] Resource usage reports

**Files**: `internal/cost/estimator.go`, `internal/cost/tracker.go`

#### 3.3 Advanced Deployment Strategies (Week 12-13)
- [ ] Implement canary deployments
  - Traffic splitting with Istio or nginx-ingress
  - Automated rollback on metrics threshold
- [ ] Implement A/B testing support
- [ ] Add deployment scheduling
- [ ] Implement maintenance windows
- [ ] Add deployment approvals workflow

**Files**: `internal/deployer/canary.go`, `internal/deployer/strategies/`

#### 3.4 GitOps Integration (Week 13)
- [ ] Implement webhook receiver for Git events
- [ ] Auto-deploy on push to main branch
- [ ] Deploy preview environments for PRs
- [ ] Add GitHub App integration
- [ ] Implement deployment status checks
- [ ] Add GitHub Actions integration

**Files**: `internal/gitops/webhook.go`, `internal/gitops/github.go`

#### 3.5 CLI Tool (Week 13-14)
- [ ] Create CLI for platform management
  - `deployer deploy <repo-url>` - Deploy app
  - `deployer list` - List deployments
  - `deployer logs <id>` - Stream logs
  - `deployer destroy <id>` - Destroy deployment
  - `deployer rollback <id>` - Rollback deployment
- [ ] Add interactive mode for configuration
- [ ] Implement config file support (~/.deployer.yaml)
- [ ] Add shell completion (bash, zsh, fish)

**Files**: `cmd/cli/main.go`, `internal/cli/`

#### 3.6 Self-Hosting (Week 14)
- [ ] Create Kubernetes manifests for platform components
  - API deployment
  - Worker deployment
  - PostgreSQL StatefulSet
  - Redis deployment
- [ ] Create Helm chart for platform installation
- [ ] Write installation documentation
- [ ] Add upgrade procedures
- [ ] Test on multiple Kubernetes distributions
  - EKS, GKE, AKS
  - k3s, microk8s
  - OpenShift

**Files**: `deployments/kubernetes/`, `deployments/helm/app-deployer/`

#### 3.7 Testing & Documentation (Week 14)
- [ ] Achieve 80%+ code coverage
- [ ] Write comprehensive E2E tests
- [ ] Chaos engineering tests (pod failures, network issues)
- [ ] Performance testing and optimization
- [ ] Security penetration testing
- [ ] Complete API documentation
- [ ] Create user guides and tutorials
- [ ] Create architecture decision records (ADRs)

**Deliverables**:
- Production-ready platform
- Multi-tenant support with RBAC
- Cost optimization features
- Self-hosting capability
- Comprehensive documentation

---

## Phase 4: Advanced Features (Week 15-20)

### Goals
- Edge deployments
- Database provisioning
- Marketplace and templates
- Advanced analytics

### Tasks

#### 4.1 Edge & Serverless (Week 15-16)
- [ ] Add Cloudflare Workers support
- [ ] Add AWS Lambda support
- [ ] Add Vercel/Netlify integration for static sites
- [ ] Add Cloud Functions (GCP) support
- [ ] Add Azure Functions support

**Files**: `internal/deployer/serverless/`

#### 4.2 Database Provisioning (Week 16-17)
- [ ] Auto-provision managed databases
  - AWS RDS (PostgreSQL, MySQL)
  - GCP Cloud SQL
  - Azure Database
- [ ] Implement database backup strategies
- [ ] Add database migration support
- [ ] Connection string management

**Files**: `internal/provisioner/database/`

#### 4.3 Marketplace & Templates (Week 17-18)
- [ ] Create template system for common stacks
  - MERN (MongoDB, Express, React, Node)
  - Django + PostgreSQL
  - Rails + PostgreSQL
  - Go + PostgreSQL
- [ ] Implement one-click deployments from templates
- [ ] Add community template submission
- [ ] Template versioning and updates

**Files**: `templates/marketplace/`, `internal/marketplace/`

#### 4.4 Analytics & Insights (Week 18-19)
- [ ] Implement deployment analytics dashboard
- [ ] Track deployment trends
- [ ] Resource utilization insights
- [ ] Cost optimization recommendations
- [ ] Performance benchmarking
- [ ] Anomaly detection

**Files**: `internal/analytics/`

#### 4.5 Enterprise Features (Week 19-20)
- [ ] SSO integration (SAML, OAuth)
- [ ] Audit logging
- [ ] Compliance reporting (SOC2, GDPR)
- [ ] Private registry support
- [ ] Air-gapped deployment support
- [ ] Custom buildpack support
- [ ] Advanced networking (VPN, private link)

**Files**: `internal/enterprise/`

**Deliverables**:
- Edge and serverless deployment support
- Database provisioning
- Template marketplace
- Enterprise-grade features

---

## Milestones Summary

| Phase | Duration | Key Deliverable | Status |
|-------|----------|-----------------|--------|
| **Phase 0** | Week 1-2 | Development environment setup | ✅ **Complete** |
| **Phase 1** | Week 3-6 | MVP (GCP only, single tenant) | 🔄 **In Progress (90%)** |
| **Phase 2** | Week 7-10 | Multi-cloud, security, observability | 🔄 **In Progress (50%)** |
| **Phase 3** | Week 11-14 | Production-ready, multi-tenant | ⏳ Pending |
| **Phase 4** | Week 15-20 | Advanced features | ⏳ Pending |

### Current Status Details (Phase 1)

**✅ Completed:**
- 1.1 Database & State Management (100%)
- 1.2 API Layer (90% - missing Swagger docs & some integration tests)
- 1.3 Repository Analyzer (90% - missing integration tests with real repos)
- 1.4 Image Builder (85% - missing integration tests & timeout handling)
- 1.5 Infrastructure Provisioner - GCP (95% - missing integration tests)
  - VPC, Subnet, Router, NAT
  - GKE cluster with node pools
  - Service accounts with IAM bindings
  - Firewall rules
  - Pulumi Automation API integration
  - Infrastructure state tracking
- 1.6 Kubernetes Deployer (90% - missing integration tests)
  - Helm chart deployment
  - Pod health checking
  - LoadBalancer IP retrieval
  - Rollback support
- 1.7 Orchestrator (100%)
  - Redis job queue (Build, Provision, Deploy, Destroy, Rollback jobs)
  - Concurrent worker processing with round-robin
  - Retry logic with exponential backoff
  - Auto-rollback on failed deployments
  - Per-phase deployment logging
  - E2E tests added

**🔄 In Progress:**
- Sample app deployments validation
- Integration tests for remaining components

**⏳ Next Up:**
- 1.8 MVP Testing & Validation (0%)
  - Deploy sample Node.js/Python apps
  - Verify external URL accessibility
  - Test deployment lifecycle (create, update, rollback, destroy)
  - Load testing

**Recent Commits:**
- `ad51023` - remove redundant lint and test workflows
- `dd43391` - add unified CI workflow for all branches
- `e6e4e94` - add E2E tests and enhance orchestrator infrastructure
- `9b52522` - fix GKE provisioning and auth issues for E2E tests
- `082733a` - fix critical implementation gaps in deployer and orchestrator

### Current Status Details (Phase 2)

**✅ Completed:**
- 2.2 Security Enhancements (60%)
  - JWT authentication with user registration/login
  - API key authentication with scopes
  - IP-based rate limiting with token bucket algorithm
  - Trivy vulnerability scanning integration
  - Image scanning before deployment (configurable fail on critical/high)
- 2.3 Observability (90%)
  - Prometheus metrics for all components
  - HTTP request metrics middleware
  - Grafana dashboard with comprehensive panels
  - Health check endpoints (/health/live, /health/ready, /metrics)
  - Docker Compose for local monitoring stack
  - OpenTelemetry distributed tracing with OTLP exporter
  - Jaeger integration for trace visualization
  - HTTP tracing middleware with context propagation

**🔄 In Progress:**
- Multi-cloud support (AWS/Azure provisioners)
- Log aggregation setup (Loki)

**⏳ Next Up:**
- 2.4 Rollback & Recovery
- 2.5 Advanced Features (auto-scaling, custom domains, DNS)

**Recent Feature Branches:**
- `feature/jwt-auth` - Security features (JWT, rate limiting, Trivy scanning)
- `feature/observability` - Prometheus metrics, Grafana dashboards

---

## Development Principles

### Start Simple, Iterate
- Build the simplest version that works
- Add complexity only when needed
- Test each component in isolation before integration

### Security First
- Never commit secrets
- Validate all inputs
- Use least privilege everywhere
- Encrypt sensitive data at rest and in transit

### Observability from Day One
- Log all important events
- Add metrics for all operations
- Trace requests end-to-end
- Monitor resource usage

### Fail Fast, Recover Gracefully
- Validate early in the pipeline
- Implement circuit breakers
- Make all operations idempotent
- Always provide rollback capability

### Cost Awareness
- Track costs per deployment
- Set resource limits
- Use spot instances where appropriate
- Implement auto-cleanup of idle resources

---

## Testing Strategy

### Unit Tests
- Test each package in isolation
- Mock external dependencies
- Aim for 80%+ coverage
- Fast execution (< 1 second per package)

### Integration Tests
- Test component interactions
- Use real dependencies (Docker containers)
- Test against real cloud providers (in test accounts)
- Longer execution acceptable (< 5 minutes)

### End-to-End Tests
- Test complete deployment flows
- Use real repositories and applications
- Verify external accessibility
- Run in CI on every PR to main
- Clean up all resources after test

### Performance Tests
- Load testing (concurrent deployments)
- Stress testing (resource limits)
- Soak testing (long-running stability)
- Benchmark critical paths

### Security Tests
- Vulnerability scanning of dependencies
- Container image scanning
- Infrastructure security scanning (tfsec, checkov)
- Penetration testing before production

---

## Success Metrics

### MVP Success Criteria (Phase 1)
- [ ] Deploy a Node.js app from GitHub URL to GCP GKE
- [ ] Return external IP in < 15 minutes
- [ ] Application accessible and healthy
- [ ] Clean resource destruction
- [ ] Zero manual intervention required

### Production Success Criteria (Phase 3)
- [ ] Support AWS, GCP, Azure
- [ ] 99% deployment success rate
- [ ] < 15 minute average deployment time
- [ ] Zero security vulnerabilities (high/critical)
- [ ] Self-hostable on any Kubernetes cluster
- [ ] Comprehensive documentation

### Scale Targets (Phase 4+)
- [ ] 100 concurrent deployments
- [ ] 1000 total active deployments
- [ ] 99.9% platform uptime
- [ ] < 5 minute rollback time
- [ ] < $0.50 per deployment in infrastructure costs

---

## Risk Mitigation

### Technical Risks
- **Risk**: Cloud provider API changes
  - **Mitigation**: Pin SDK versions, extensive integration tests
- **Risk**: Pulumi state corruption
  - **Mitigation**: Regular backups, state locking, validation
- **Risk**: Kubernetes cluster failures
  - **Mitigation**: Multi-AZ deployments, regular backups, disaster recovery plan

### Operational Risks
- **Risk**: Cost overrun from runaway resources
  - **Mitigation**: Resource quotas, budget alerts, auto-cleanup
- **Risk**: Security breach
  - **Mitigation**: Regular security audits, penetration testing, incident response plan
- **Risk**: Data loss
  - **Mitigation**: Automated backups, multi-region replication, tested recovery procedures

### Project Risks
- **Risk**: Scope creep
  - **Mitigation**: Strict phase adherence, MVP-first mentality
- **Risk**: Technology choice regret
  - **Mitigation**: Proof of concepts for critical decisions, modular architecture for swappability

---

## Next Steps

1. **Review and refine** this roadmap based on your specific requirements
2. **Set up development environment** (Phase 0)
3. **Create detailed task breakdown** for Phase 1 in GitHub Issues/Projects
4. **Begin MVP development** with Repository Analyzer
5. **Establish weekly progress reviews** to stay on track

---

## Questions to Resolve Before Starting

- [x] Which cloud provider should we prioritize for MVP? **GCP (free tier for 1 year)**
- [x] Do you have existing cloud accounts for testing? **Yes, GCP test accounts available**
- [x] What's the target deployment scale for MVP testing? **3-5 concurrent deployments**
- [x] Should we use existing Kubernetes cluster or provision new ones? **New GKE clusters**
- [x] What's the preferred container registry for MVP? **Open-source (GCP Artifact Registry free tier or Harbor)**
- [x] Do you have domain names available for testing DNS/SSL? **No - use external IP for now, DNS service later (detached architecture)**
- [x] What's the acceptable cost for MVP testing? **GCP free tier (1 year), plan budget after**
