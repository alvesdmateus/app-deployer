# Architecture Design

## System Overview

**app-deployer** is a custom PaaS platform that transforms a repository URL into a fully deployed, accessible application with automated infrastructure provisioning.

```
┌─────────────────────────────────────────────────────────────────────┐
│                          app-deployer Platform                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  Input: Repository URL                                               │
│     ↓                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌─────────────────┐       │
│  │   API Layer  │ →  │ Orchestrator │ →  │  State Manager  │       │
│  └──────────────┘    └──────────────┘    └─────────────────┘       │
│                            ↓                                          │
│              ┌─────────────┼─────────────┐                          │
│              ↓             ↓             ↓                           │
│     ┌────────────┐  ┌────────────┐  ┌────────────┐                 │
│     │  Analyzer  │  │  Builder   │  │ Provisioner│                 │
│     └────────────┘  └────────────┘  └────────────┘                 │
│              ↓             ↓             ↓                           │
│              └─────────────┼─────────────┘                          │
│                            ↓                                          │
│                    ┌──────────────┐                                  │
│                    │   Deployer   │                                  │
│                    └──────────────┘                                  │
│                            ↓                                          │
│  Output: External URL + IP                                           │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. API Layer
**Purpose**: External interface for triggering deployments and querying status

**Technology**: Go with Fiber/Echo or Python with FastAPI
- RESTful API endpoints
- WebSocket for real-time deployment logs
- Authentication/authorization (JWT)
- Rate limiting and request validation

**Key Endpoints**:
- `POST /deployments` - Initiate deployment from repo URL
- `GET /deployments/{id}` - Get deployment status
- `GET /deployments/{id}/logs` - Stream deployment logs
- `DELETE /deployments/{id}` - Destroy deployment and infrastructure
- `POST /deployments/{id}/rollback` - Rollback to previous version

### 2. Orchestrator (Core Engine)
**Purpose**: Coordinates all deployment phases and manages workflow state

**Technology**: Go (for concurrency, performance, reliability)
- State machine for deployment lifecycle
- Async task queue (using Redis + BullMQ or Go channels)
- Error handling and retry logic with exponential backoff
- Event emission for observability

**Deployment States**:
```
QUEUED → ANALYZING → BUILDING → PROVISIONING → DEPLOYING → HEALTHY → EXPOSED
                                        ↓
                                     FAILED → ROLLING_BACK
```

### 3. Repository Analyzer
**Purpose**: Clone, analyze, and detect application requirements

**Technology**: Go with git libraries + language detection
- Git clone with shallow copy
- Language/framework detection using **Nixpacks** logic or custom detectors
- Parse config files (package.json, requirements.txt, go.mod, etc.)
- Environment variable extraction
- Resource requirement estimation (CPU, memory)

**Output**:
```json
{
  "language": "node",
  "framework": "express",
  "version": "18",
  "buildCommand": "npm install && npm run build",
  "startCommand": "npm start",
  "port": 3000,
  "envVars": ["DATABASE_URL", "API_KEY"],
  "resources": {
    "cpu": "500m",
    "memory": "512Mi"
  }
}
```

### 4. Image Builder
**Purpose**: Build container images from source code

**Technology**:
- **Cloud Native Buildpacks** (pack CLI) - for automatic buildpack detection
- **Nixpacks** (Rust-based) - Railway's open-source builder
- Fallback to Dockerfile if present
- **Buildah** or **Kaniko** for rootless builds in Kubernetes

**Features**:
- Multi-stage builds for optimization
- Layer caching for faster rebuilds
- Image signing and SBOM generation
- Push to container registry (Harbor, ECR, GCR, or self-hosted)
- Vulnerability scanning with Trivy

**Registry Strategy**:
- Self-hosted Harbor for full control
- Cloud provider registries for integration (ECR, GCR, ACR)
- Per-deployment image tagging: `app-name:git-sha` and `app-name:latest`

### 5. Infrastructure Provisioner
**Purpose**: Create and manage cloud infrastructure

**Technology**: **Pulumi Automation API** (Go or Python SDK)

**Why Pulumi over Terraform**:
- Programmatic API (no CLI shelling)
- Real programming languages (type safety, testing)
- Better state management options
- Built-in secret encryption

**Provisioned Resources**:
- **Cloud Provider Project/Account** (if needed)
- **VPC** with public/private subnets
- **Security Groups** (minimal exposure, only required ports)
- **Kubernetes Cluster** (EKS, GKE, AKS) or single-node k3s for cost efficiency
- **Load Balancer** (cloud-native or nginx-ingress)
- **DNS Records** (Route53, Cloud DNS, or external)
- **SSL Certificates** (cert-manager with Let's Encrypt)

**State Management**:
- Pulumi state backend (S3, GCS, Azure Blob, or Pulumi Cloud)
- Per-deployment isolated stacks
- Encryption at rest
- State locking for concurrent safety

**Infrastructure Templates**:
```
templates/
├── aws/
│   ├── vpc.go
│   ├── eks.go
│   └── loadbalancer.go
├── gcp/
│   ├── vpc.go
│   ├── gke.go
│   └── loadbalancer.go
└── azure/
    ├── vnet.go
    ├── aks.go
    └── loadbalancer.go
```

### 6. Kubernetes Deployer
**Purpose**: Deploy applications to Kubernetes with auto-generated manifests

**Technology**:
- **Helm** - package management and templating
- Go Kubernetes client (client-go)
- Custom Helm chart generator

**Generated Resources**:
- Deployment with health checks, resource limits, and restart policies
- Service (ClusterIP internally, LoadBalancer for external)
- Ingress with TLS termination
- ConfigMap for configuration
- Secret for sensitive data (sealed-secrets or external-secrets)
- HorizontalPodAutoscaler for auto-scaling
- NetworkPolicy for isolation

**Helm Chart Structure**:
```yaml
# values.yaml (auto-generated)
image:
  repository: registry.example.com/app
  tag: abc123
  pullPolicy: IfNotPresent

service:
  type: LoadBalancer
  port: 80
  targetPort: 3000

ingress:
  enabled: true
  host: app.example.com
  tls: true

resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilization: 70
```

### 7. State Manager
**Purpose**: Track deployments, infrastructure state, and application metadata

**Technology**: PostgreSQL with GORM (Go) or SQLAlchemy (Python)

**Schema**:
```sql
-- Deployments table
CREATE TABLE deployments (
    id UUID PRIMARY KEY,
    repo_url VARCHAR NOT NULL,
    git_sha VARCHAR NOT NULL,
    status VARCHAR NOT NULL, -- QUEUED, BUILDING, DEPLOYING, HEALTHY, FAILED
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deployed_at TIMESTAMP,
    external_url VARCHAR,
    external_ip VARCHAR,
    cloud_provider VARCHAR,
    region VARCHAR,
    metadata JSONB -- flexible storage for additional data
);

-- Infrastructure state references
CREATE TABLE infrastructure (
    id UUID PRIMARY KEY,
    deployment_id UUID REFERENCES deployments(id),
    pulumi_stack_name VARCHAR,
    resources JSONB, -- list of created resources
    state_backend_url VARCHAR,
    created_at TIMESTAMP
);

-- Build history
CREATE TABLE builds (
    id UUID PRIMARY KEY,
    deployment_id UUID REFERENCES deployments(id),
    image_tag VARCHAR,
    build_logs TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    success BOOLEAN
);
```

### 8. Observability Layer
**Purpose**: Monitoring, logging, and alerting

**Technology**:
- **Prometheus** - Metrics collection
- **Grafana** - Dashboards
- **Loki** - Log aggregation
- **Jaeger** - Distributed tracing
- **AlertManager** - Alert routing

**Metrics Tracked**:
- Deployment success/failure rate
- Build time (p50, p95, p99)
- Infrastructure provisioning time
- Application health status
- Resource utilization per deployment
- API request latency

## Data Flow

### Deployment Lifecycle

```
1. API Request
   POST /deployments {"repo_url": "https://github.com/user/app"}
   ↓

2. Orchestrator
   - Create deployment record (status: QUEUED)
   - Queue async job
   - Return deployment ID to client
   ↓

3. Analyzer Phase
   - Clone repository
   - Detect language/framework
   - Parse configuration
   - Estimate resources
   - Update status: ANALYZING → BUILDING
   ↓

4. Builder Phase
   - Generate Dockerfile or use buildpack
   - Build container image
   - Scan for vulnerabilities
   - Push to registry
   - Update status: BUILDING → PROVISIONING
   ↓

5. Provisioner Phase
   - Initialize Pulumi stack
   - Create VPC, subnets, security groups
   - Provision Kubernetes cluster (or use existing)
   - Create load balancer
   - Setup DNS and SSL
   - Update status: PROVISIONING → DEPLOYING
   ↓

6. Deployer Phase
   - Generate Helm chart
   - Deploy to Kubernetes
   - Wait for pods to be healthy
   - Verify service accessibility
   - Update status: DEPLOYING → HEALTHY
   ↓

7. Exposure Phase
   - Get LoadBalancer external IP
   - Update DNS records
   - Return external URL
   - Update status: HEALTHY → EXPOSED
   ↓

8. Response
   {
     "id": "uuid",
     "status": "EXPOSED",
     "url": "https://app.example.com",
     "ip": "203.0.113.42",
     "deployed_at": "2026-01-04T10:30:00Z"
   }
```

## Technology Stack Summary

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **API Layer** | Go (Fiber) | Performance, concurrency, type safety |
| **Orchestrator** | Go | Excellent concurrency primitives, reliability |
| **Analyzer** | Go + Nixpacks integration | Fast execution, existing tooling |
| **Builder** | Cloud Native Buildpacks/Nixpacks | Auto-detection, security, best practices |
| **Provisioner** | Pulumi Automation API (Go SDK) | Programmatic, type-safe IaC |
| **Deployer** | Helm + client-go | Industry standard, flexible templating |
| **Database** | PostgreSQL | ACID, reliability, JSONB for flexibility |
| **Cache/Queue** | Redis | Fast, reliable, pub/sub capabilities |
| **Registry** | Harbor | Open-source, security scanning, replication |
| **Observability** | Prometheus + Grafana + Loki | Cloud-native standard, comprehensive |

## Security Architecture

### Defense in Depth

**1. API Layer**
- JWT authentication with short-lived tokens
- API key rotation
- Rate limiting per user/IP
- Input validation and sanitization
- HTTPS only (TLS 1.3)

**2. Secrets Management**
- Never store secrets in code or state
- Use cloud provider secret managers (AWS Secrets Manager, GCP Secret Manager)
- Or HashiCorp Vault for multi-cloud
- Inject secrets at runtime via Kubernetes external-secrets

**3. Container Security**
- Non-root container users
- Read-only root filesystems
- Dropped capabilities
- Image vulnerability scanning (Trivy, Snyk)
- Image signing with Sigstore/cosign
- SBOM generation

**4. Network Security**
- VPC isolation per deployment (optional, for multi-tenant)
- Network policies in Kubernetes
- Security groups with minimal ports open
- Private subnets for workloads
- Bastion hosts for SSH access (if needed)

**5. Infrastructure Security**
- Least privilege IAM roles
- Encryption at rest (EBS, persistent volumes)
- Encryption in transit
- Audit logging (CloudTrail, GCP Audit Logs)

**6. Supply Chain Security**
- Pin buildpack versions
- Verify image signatures
- Scan dependencies for vulnerabilities
- Use private registries

## Scalability Design

### Horizontal Scaling
- **API Layer**: Stateless, behind load balancer, auto-scale on CPU/memory
- **Orchestrator**: Job queue workers, scale workers based on queue depth
- **Database**: Read replicas for queries, write scaling with partitioning

### Multi-Tenancy
- **Namespace Isolation**: One Kubernetes namespace per deployment
- **Resource Quotas**: CPU/memory limits per namespace
- **Network Policies**: Prevent cross-deployment traffic
- **Cost Allocation**: Track costs per deployment using labels

### Cost Optimization
- **Cluster Sharing**: Multiple apps on shared Kubernetes cluster
- **Node Auto-scaling**: Scale cluster nodes based on demand
- **Spot Instances**: Use for non-production workloads
- **Resource Limits**: Prevent runaway resource consumption
- **Idle Shutdown**: Stop inactive deployments after N hours

## Reliability Patterns

### Idempotency
- All operations are idempotent (safe to retry)
- Use unique deployment IDs for deduplication
- Check current state before applying changes

### Graceful Degradation
- If Kubernetes provisioning fails, fall back to Docker Compose on VM
- If cloud provider unavailable, queue deployment for retry
- Partial failures don't block entire deployment

### Health Checks
- Application readiness and liveness probes
- Infrastructure health checks (API endpoints, database connectivity)
- Automated rollback on failed health checks

### Rollback Strategy
- Keep previous deployment version in state
- One-command rollback: `POST /deployments/{id}/rollback`
- Restore previous Helm release
- Keep previous infrastructure state for quick recovery

### Disaster Recovery
- Regular database backups (automated, encrypted)
- Infrastructure state backups (Pulumi state)
- Multi-region registry replication
- Documented recovery procedures

## Configuration Management

### Platform Configuration
**config.yaml** - Platform-level settings
```yaml
platform:
  name: app-deployer
  default_region: us-east-1
  default_cloud: aws

registry:
  type: harbor
  url: https://registry.example.com
  credentials_secret: registry-creds

database:
  host: postgres.internal
  port: 5432
  name: app_deployer
  ssl_mode: require

observability:
  metrics_enabled: true
  tracing_enabled: true
  log_level: info

limits:
  max_deployments_per_user: 10
  max_cpu_per_deployment: 4000m
  max_memory_per_deployment: 8Gi
  max_build_time: 30m
```

### Deployment Configuration
**deployer.yaml** - Per-deployment overrides (optional)
```yaml
deployment:
  cloud_provider: aws  # or gcp, azure
  region: us-west-2
  kubernetes:
    cluster_type: eks  # or gke, aks, k3s
    node_type: t3.medium
    min_nodes: 2
    max_nodes: 10

  application:
    port: 3000
    env_vars:
      NODE_ENV: production
      LOG_LEVEL: info
    resources:
      cpu: 1000m
      memory: 1Gi
    replicas: 3
    autoscaling: true

  networking:
    domain: myapp.example.com
    ssl: true
    ssl_provider: letsencrypt
```

## Development Workflow

### Repository Structure
```
app-deployer/
├── cmd/
│   ├── api/           # API server entry point
│   ├── worker/        # Orchestrator worker entry point
│   └── cli/           # CLI tool for management
├── internal/
│   ├── api/           # API handlers and routes
│   ├── orchestrator/  # Core orchestration logic
│   ├── analyzer/      # Repository analysis
│   ├── builder/       # Image building
│   ├── provisioner/   # Infrastructure provisioning
│   ├── deployer/      # Kubernetes deployment
│   ├── state/         # State management (DB models)
│   └── observability/ # Logging, metrics, tracing
├── pkg/
│   ├── models/        # Shared data models
│   ├── utils/         # Shared utilities
│   └── config/        # Configuration loading
├── deployments/
│   └── kubernetes/    # Platform self-hosting manifests
├── templates/
│   ├── pulumi/        # Infrastructure templates
│   └── helm/          # Application Helm charts
├── scripts/
│   └── setup/         # Setup and installation scripts
├── docs/              # Documentation
├── tests/
│   ├── unit/
│   ├── integration/
│   └── e2e/
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── docker-compose.yml # Local development stack
├── config.yaml
└── README.md
```

## Future Enhancements

### Phase 2 Features
- **Multi-region deployments** - Deploy to multiple regions simultaneously
- **Blue-green deployments** - Zero-downtime updates
- **Canary releases** - Gradual traffic shifting
- **GitOps integration** - Watch repository for changes, auto-deploy
- **Cost estimation** - Predict infrastructure costs before deployment
- **Custom domains** - Bring your own domain support
- **Database provisioning** - Auto-provision managed databases
- **Secrets UI** - Web interface for managing secrets
- **Team collaboration** - Organizations, RBAC, audit logs

### Phase 3 Features
- **Edge deployments** - Deploy to edge locations (Cloudflare Workers, Lambda@Edge)
- **Serverless support** - Deploy to Lambda, Cloud Functions, Azure Functions
- **Terraform support** - Alternative to Pulumi for users preferring HCL
- **Preview environments** - Auto-deploy PR branches
- **ChatOps** - Deploy via Slack/Discord commands
- **Marketplace** - Pre-configured stacks (MERN, Django, Rails, etc.)
