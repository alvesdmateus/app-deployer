#!/bin/bash

# Simple API test script
BASE_URL="http://localhost:3000"

echo "=== Testing app-deployer API ==="
echo ""

# Test health check
echo "1. Testing health check..."
curl -s "${BASE_URL}/health" | jq .
echo ""

# Create a deployment
echo "2. Creating a deployment..."
DEPLOYMENT=$(curl -s -X POST "${BASE_URL}/api/v1/deployments" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-deployment",
    "app_name": "my-app",
    "version": "v1.0.0",
    "cloud": "gcp",
    "region": "us-central1"
  }')
echo "$DEPLOYMENT" | jq .
DEPLOYMENT_ID=$(echo "$DEPLOYMENT" | jq -r '.id')
echo "Deployment ID: $DEPLOYMENT_ID"
echo ""

# Get the deployment
echo "3. Getting deployment by ID..."
curl -s "${BASE_URL}/api/v1/deployments/${DEPLOYMENT_ID}" | jq .
echo ""

# List all deployments
echo "4. Listing all deployments..."
curl -s "${BASE_URL}/api/v1/deployments?limit=10&offset=0" | jq .
echo ""

# Update deployment status
echo "5. Updating deployment status..."
curl -s -X PATCH "${BASE_URL}/api/v1/deployments/${DEPLOYMENT_ID}/status" \
  -H "Content-Type: application/json" \
  -d '{"status": "BUILDING"}' | jq .
echo ""

# Get deployments by status
echo "6. Getting deployments by status (BUILDING)..."
curl -s "${BASE_URL}/api/v1/deployments/status/BUILDING" | jq .
echo ""

# Delete the deployment
echo "7. Deleting deployment..."
curl -s -X DELETE "${BASE_URL}/api/v1/deployments/${DEPLOYMENT_ID}" | jq .
echo ""

echo "=== API tests completed ==="
