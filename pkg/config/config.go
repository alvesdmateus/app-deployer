package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Platform    PlatformConfig
	Registry    RegistryConfig
	Provisioner ProvisionerConfig
	Deployer    DeployerConfig
	Worker      WorkerConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	LogLevel     string
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL      string
	Password string
	DB       int
}

// PlatformConfig holds platform-wide settings
type PlatformConfig struct {
	DefaultCloud  string
	DefaultRegion string
}

// RegistryConfig holds container registry configuration
type RegistryConfig struct {
	Type     string
	Project  string
	Location string
	URL      string
}

// ProvisionerConfig holds infrastructure provisioner configuration
type ProvisionerConfig struct {
	GCPProject      string
	GCPRegion       string
	PulumiBackend   string
	DefaultNodeType string
	DefaultNodes    int
}

// DeployerConfig holds Kubernetes deployer configuration
type DeployerConfig struct {
	DefaultReplicas int
	DefaultPort     int
	HelmTimeout     time.Duration
	PodTimeout      time.Duration
}

// WorkerConfig holds orchestrator worker configuration
type WorkerConfig struct {
	Concurrency  int
	PollInterval time.Duration
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set defaults
	setDefaults()

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, use defaults and env vars only
	}

	// Override with environment variables
	viper.AutomaticEnv()

	config := &Config{
		Server: ServerConfig{
			Port:         viper.GetString("server.port"),
			ReadTimeout:  viper.GetDuration("server.read_timeout"),
			WriteTimeout: viper.GetDuration("server.write_timeout"),
			LogLevel:     viper.GetString("server.log_level"),
		},
		Database: DatabaseConfig{
			Host:            viper.GetString("database.host"),
			Port:            viper.GetInt("database.port"),
			User:            viper.GetString("database.user"),
			Password:        viper.GetString("database.password"),
			DBName:          viper.GetString("database.dbname"),
			SSLMode:         viper.GetString("database.sslmode"),
			MaxOpenConns:    viper.GetInt("database.max_open_conns"),
			MaxIdleConns:    viper.GetInt("database.max_idle_conns"),
			ConnMaxLifetime: viper.GetDuration("database.conn_max_lifetime"),
		},
		Redis: RedisConfig{
			URL:      viper.GetString("redis.url"),
			Password: viper.GetString("redis.password"),
			DB:       viper.GetInt("redis.db"),
		},
		Platform: PlatformConfig{
			DefaultCloud:  viper.GetString("platform.default_cloud"),
			DefaultRegion: viper.GetString("platform.default_region"),
		},
		Registry: RegistryConfig{
			Type:     viper.GetString("registry.type"),
			Project:  viper.GetString("registry.project"),
			Location: viper.GetString("registry.location"),
			URL:      viper.GetString("registry.url"),
		},
		Provisioner: ProvisionerConfig{
			GCPProject:      viper.GetString("provisioner.gcp_project"),
			GCPRegion:       viper.GetString("provisioner.gcp_region"),
			PulumiBackend:   viper.GetString("provisioner.pulumi_backend"),
			DefaultNodeType: viper.GetString("provisioner.default_node_type"),
			DefaultNodes:    viper.GetInt("provisioner.default_nodes"),
		},
		Deployer: DeployerConfig{
			DefaultReplicas: viper.GetInt("deployer.default_replicas"),
			DefaultPort:     viper.GetInt("deployer.default_port"),
			HelmTimeout:     viper.GetDuration("deployer.helm_timeout"),
			PodTimeout:      viper.GetDuration("deployer.pod_timeout"),
		},
		Worker: WorkerConfig{
			Concurrency:  viper.GetInt("worker.concurrency"),
			PollInterval: viper.GetDuration("worker.poll_interval"),
		},
	}

	// Override database config from DATABASE_URL if present
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		// DATABASE_URL takes precedence
		viper.Set("database.url", dbURL)
	}

	return config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "3000")
	viper.SetDefault("server.read_timeout", 10*time.Second)
	viper.SetDefault("server.write_timeout", 10*time.Second)
	viper.SetDefault("server.log_level", "info")

	// Database defaults
	viper.SetDefault("database.host", "127.0.0.1")
	viper.SetDefault("database.port", 5434)
	viper.SetDefault("database.user", "deployer")
	viper.SetDefault("database.password", "deployer_dev_password")
	viper.SetDefault("database.dbname", "app_deployer")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 5*time.Minute)

	// Redis defaults
	viper.SetDefault("redis.url", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Platform defaults
	viper.SetDefault("platform.default_cloud", "gcp")
	viper.SetDefault("platform.default_region", "us-central1")

	// Registry defaults
	viper.SetDefault("registry.type", "artifact-registry")
	viper.SetDefault("registry.project", "")
	viper.SetDefault("registry.location", "us-central1")
	viper.SetDefault("registry.url", "")

	// Provisioner defaults
	viper.SetDefault("provisioner.gcp_project", "")
	viper.SetDefault("provisioner.gcp_region", "us-central1")
	viper.SetDefault("provisioner.pulumi_backend", "")
	viper.SetDefault("provisioner.default_node_type", "e2-small")
	viper.SetDefault("provisioner.default_nodes", 2)

	// Deployer defaults
	viper.SetDefault("deployer.default_replicas", 2)
	viper.SetDefault("deployer.default_port", 8080)
	viper.SetDefault("deployer.helm_timeout", 5*time.Minute)
	viper.SetDefault("deployer.pod_timeout", 5*time.Minute)

	// Worker defaults
	viper.SetDefault("worker.concurrency", 3)
	viper.SetDefault("worker.poll_interval", 5*time.Second)
}

// GetDatabaseDSN returns the PostgreSQL connection string
func (c *Config) GetDatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}
