# API Documentation

## Base URL

```
http://localhost:3000/api/v1
```

## Health Check

### Check API Health

```http
GET /health
```

**Response:**
```json
{
  "status": "ok",
  "database": "ok",
  "version": "1.0.0"
}
```

## Deployments

### Create Deployment

Create a new deployment.

```http
POST /api/v1/deployments
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "my-deployment",
  "app_name": "my-app",
  "version": "v1.0.0",
  "cloud": "gcp",
  "region": "us-central1"
}
```

**Response:** `201 Created`
```json
{
  "id": "uuid",
  "name": "my-deployment",
  "app_name": "my-app",
  "version": "v1.0.0",
  "status": "PENDING",
  "cloud": "gcp",
  "region": "us-central1",
  "created_at": "2026-01-04T12:00:00Z",
  "updated_at": "2026-01-04T12:00:00Z"
}
```

### Get Deployment

Retrieve a specific deployment by ID.

```http
GET /api/v1/deployments/{id}
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "name": "my-deployment",
  "app_name": "my-app",
  "version": "v1.0.0",
  "status": "PENDING",
  "cloud": "gcp",
  "region": "us-central1",
  "external_ip": "",
  "external_url": "",
  "created_at": "2026-01-04T12:00:00Z",
  "updated_at": "2026-01-04T12:00:00Z"
}
```

### List Deployments

List all deployments with pagination.

```http
GET /api/v1/deployments?limit=20&offset=0
```

**Query Parameters:**
- `limit` (optional): Number of results per page (default: 20)
- `offset` (optional): Number of results to skip (default: 0)

**Response:** `200 OK`
```json
{
  "deployments": [
    {
      "id": "uuid",
      "name": "my-deployment",
      "app_name": "my-app",
      "version": "v1.0.0",
      "status": "PENDING",
      "cloud": "gcp",
      "region": "us-central1",
      "created_at": "2026-01-04T12:00:00Z",
      "updated_at": "2026-01-04T12:00:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

### Update Deployment Status

Update the status of a deployment.

```http
PATCH /api/v1/deployments/{id}/status
Content-Type: application/json
```

**Request Body:**
```json
{
  "status": "BUILDING"
}
```

**Valid Statuses:**
- `PENDING` - Initial state
- `BUILDING` - Container image is being built
- `PROVISIONING` - Infrastructure is being provisioned
- `DEPLOYING` - Application is being deployed
- `EXPOSED` - Application is deployed and accessible
- `FAILED` - Deployment failed

**Response:** `200 OK`
```json
{
  "message": "Deployment status updated"
}
```

### Delete Deployment

Delete a deployment and all related resources.

```http
DELETE /api/v1/deployments/{id}
```

**Response:** `200 OK`
```json
{
  "message": "Deployment deleted"
}
```

### Get Deployments by Status

Retrieve all deployments with a specific status.

```http
GET /api/v1/deployments/status/{status}
```

**Response:** `200 OK`
```json
[
  {
    "id": "uuid",
    "name": "my-deployment",
    "app_name": "my-app",
    "version": "v1.0.0",
    "status": "BUILDING",
    "cloud": "gcp",
    "region": "us-central1",
    "created_at": "2026-01-04T12:00:00Z",
    "updated_at": "2026-01-04T12:00:00Z"
  }
]
```

## Infrastructure

### Get Infrastructure

Get infrastructure details for a deployment.

```http
GET /api/v1/deployments/{deployment_id}/infrastructure
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "deployment_id": "uuid",
  "cluster_name": "my-cluster",
  "namespace": "default",
  "service_name": "my-service",
  "status": "READY",
  "config": "{\"type\":\"kubernetes\"}",
  "created_at": "2026-01-04T12:00:00Z",
  "updated_at": "2026-01-04T12:00:00Z"
}
```

## Builds

### Get Latest Build

Get the most recent build for a deployment.

```http
GET /api/v1/deployments/{deployment_id}/builds/latest
```

**Response:** `200 OK`
```json
{
  "id": "uuid",
  "deployment_id": "uuid",
  "image_tag": "v1.0.0",
  "status": "SUCCESS",
  "build_log": "Build output...",
  "started_at": "2026-01-04T12:00:00Z",
  "completed_at": "2026-01-04T12:05:00Z",
  "created_at": "2026-01-04T12:00:00Z",
  "updated_at": "2026-01-04T12:05:00Z"
}
```

## Source Code Analysis

### Analyze Source Code

Analyze source code from a local path to detect language, framework, and dependencies.

```http
POST /api/v1/analyze
Content-Type: application/json
```

**Request Body:**
```json
{
  "path": "/path/to/source/code"
}
```

**Response:** `200 OK`
```json
{
  "language": "nodejs",
  "framework": "express",
  "build_tool": "npm",
  "runtime": "node:18.x",
  "dependencies": {
    "express": "^4.18.0",
    "body-parser": "^1.20.0"
  },
  "dev_dependencies": {
    "nodemon": "^2.0.0"
  },
  "start_command": "npm start",
  "build_command": "npm install && npm run build",
  "port": 3000,
  "has_dockerfile": false,
  "files": ["package.json", "index.js", "routes/"],
  "confidence": 0.95
}
```

### Upload and Analyze

Upload source code files and analyze them.

```http
POST /api/v1/analyze/upload
Content-Type: multipart/form-data
```

**Form Data:**
- `source`: Source code file or archive (zip, tar.gz)

**Response:** `200 OK`
```json
{
  "language": "python",
  "framework": "flask",
  "build_tool": "pip",
  "runtime": "python:3.11",
  "dependencies": {
    "flask": "2.0.1",
    "requests": "2.26.0"
  },
  "start_command": "python app.py",
  "build_command": "pip install -r requirements.txt",
  "port": 5000,
  "has_dockerfile": false,
  "files": ["app.py", "requirements.txt"],
  "confidence": 0.95
}
```

### Get Supported Languages

Get a list of supported programming languages and frameworks.

```http
GET /api/v1/analyze/languages
```

**Response:** `200 OK`
```json
{
  "languages": [
    {
      "id": "go",
      "name": "Go",
      "frameworks": ["gin", "echo", "chi", "fiber"]
    },
    {
      "id": "nodejs",
      "name": "Node.js",
      "frameworks": ["express", "nestjs", "nextjs", "koa", "fastify"]
    },
    {
      "id": "python",
      "name": "Python",
      "frameworks": ["flask", "django", "fastapi"]
    },
    {
      "id": "java",
      "name": "Java",
      "frameworks": ["springboot", "quarkus"]
    }
  ]
}
```

### Supported Languages and Frameworks

#### Go
- **Frameworks**: Gin, Echo, Chi, Fiber
- **Build Tool**: go
- **Default Port**: 8080

#### Node.js
- **Frameworks**: Express, NestJS, Next.js, Koa, Fastify
- **Build Tools**: npm, yarn, pnpm
- **Default Port**: 3000

#### Python
- **Frameworks**: Flask, Django, FastAPI
- **Build Tools**: pip, poetry
- **Default Port**: 5000 (Flask), 8000 (Django)

#### Java
- **Frameworks**: Spring Boot, Quarkus
- **Build Tools**: Maven, Gradle
- **Default Port**: 8080

#### Others
- Rust (Cargo)
- Ruby (Bundler)
- PHP (Composer)
- .NET

## Error Responses

All endpoints may return error responses in the following format:

**400 Bad Request**
```json
{
  "error": "Bad Request",
  "message": "Invalid request body"
}
```

**404 Not Found**
```json
{
  "error": "Not Found",
  "message": "Deployment not found"
}
```

**500 Internal Server Error**
```json
{
  "error": "Internal Server Error",
  "message": "Failed to create deployment"
}
```

## Examples

### Using cURL

Create a deployment:
```bash
curl -X POST http://localhost:3000/api/v1/deployments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-app",
    "app_name": "my-application",
    "version": "v1.0.0",
    "cloud": "gcp",
    "region": "us-central1"
  }'
```

List deployments:
```bash
curl http://localhost:3000/api/v1/deployments?limit=10
```

Update status:
```bash
curl -X PATCH http://localhost:3000/api/v1/deployments/{id}/status \
  -H "Content-Type: application/json" \
  -d '{"status": "BUILDING"}'
```

### Using JavaScript (fetch)

```javascript
// Create deployment
const response = await fetch('http://localhost:3000/api/v1/deployments', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    name: 'test-app',
    app_name: 'my-application',
    version: 'v1.0.0',
    cloud: 'gcp',
    region: 'us-central1'
  })
});

const deployment = await response.json();
console.log(deployment);
```

### Using Python (requests)

```python
import requests

# Create deployment
response = requests.post(
    'http://localhost:3000/api/v1/deployments',
    json={
        'name': 'test-app',
        'app_name': 'my-application',
        'version': 'v1.0.0',
        'cloud': 'gcp',
        'region': 'us-central1'
    }
)

deployment = response.json()
print(deployment)
```
