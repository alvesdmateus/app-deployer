module github.com/yourusername/app-deployer

go 1.21

require (
	// HTTP Framework
	github.com/gofiber/fiber/v2 v2.52.0

	// Database
	gorm.io/gorm v1.25.5
	gorm.io/driver/postgres v1.5.4

	// Redis for job queue
	github.com/redis/go-redis/v9 v9.3.0

	// Pulumi for infrastructure
	github.com/pulumi/pulumi/sdk/v3 v3.99.0
	github.com/pulumi/pulumi-gcp/sdk/v7 v7.8.0

	// Kubernetes client
	k8s.io/client-go v0.29.0
	k8s.io/api v0.29.0
	k8s.io/apimachinery v0.29.0

	// Git operations
	github.com/go-git/go-git/v5 v5.11.0

	// Logging
	github.com/rs/zerolog v1.31.0

	// Configuration
	github.com/spf13/viper v1.18.2

	// UUID
	github.com/google/uuid v1.5.0

	// Testing
	github.com/stretchr/testify v1.8.4
)
