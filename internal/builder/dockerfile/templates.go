package dockerfile

import (
	"fmt"
	"strings"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
)

// LanguageTemplate defines a Dockerfile template for a specific language
type LanguageTemplate struct {
	BaseImage     string
	BuildStage    string
	RuntimeStage  string
	WorkDir       string
	BuildCommands []string
	RunCommand    string
}

// GetTemplate returns the appropriate Dockerfile template based on analysis
func GetTemplate(analysis *analyzer.AnalysisResult) (*LanguageTemplate, error) {
	switch analysis.Language {
	case analyzer.LanguageGo:
		return getGoTemplate(analysis), nil
	case analyzer.LanguageNodeJS:
		return getNodeTemplate(analysis), nil
	case analyzer.LanguagePython:
		return getPythonTemplate(analysis), nil
	case analyzer.LanguageJava:
		return getJavaTemplate(analysis), nil
	case analyzer.LanguageRust:
		return getRustTemplate(analysis), nil
	case analyzer.LanguageRuby:
		return getRubyTemplate(analysis), nil
	case analyzer.LanguagePHP:
		return getPHPTemplate(analysis), nil
	case analyzer.LanguageDotNet:
		return getDotNetTemplate(analysis), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", analysis.Language)
	}
}

// getGoTemplate returns optimized multi-stage Dockerfile for Go
func getGoTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "1.23" // Default Go version
	}

	buildCmd := analysis.BuildCommand
	if buildCmd == "" {
		buildCmd = "go build -o app ."
	}

	startCmd := analysis.StartCommand
	if startCmd == "" {
		startCmd = "./app"
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("golang:%s-alpine", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM golang:%s-alpine AS builder
WORKDIR /build
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux %s`, runtime, buildCmd),
		RuntimeStage: `# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder
COPY --from=builder /build/app .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser`,
		WorkDir:    "/app",
		RunCommand: startCmd,
	}
}

// getNodeTemplate returns optimized multi-stage Dockerfile for Node.js
func getNodeTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "20" // Default Node version
	}

	buildTool := string(analysis.BuildTool)
	if buildTool == "" {
		buildTool = "npm"
	}

	var installCmd, buildCmd string
	switch buildTool {
	case "yarn":
		installCmd = "yarn install --frozen-lockfile"
		buildCmd = "yarn build"
	case "pnpm":
		installCmd = "pnpm install --frozen-lockfile"
		buildCmd = "pnpm build"
	default: // npm
		installCmd = "npm ci"
		buildCmd = "npm run build"
	}

	// Use custom build command if provided
	if analysis.BuildCommand != "" {
		buildCmd = analysis.BuildCommand
	}

	startCmd := analysis.StartCommand
	if startCmd == "" {
		startCmd = "node server.js"
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("node:%s-alpine", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM node:%s-alpine AS builder
WORKDIR /build

# Copy package files
COPY package*.json yarn.lock* pnpm-lock.yaml* ./

# Install dependencies
RUN %s

# Copy source code
COPY . .

# Build application
RUN %s`, runtime, installCmd, buildCmd),
		RuntimeStage: fmt.Sprintf(`# Runtime stage
FROM node:%s-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy built application
COPY --from=builder /build/dist ./dist
COPY --from=builder /build/node_modules ./node_modules
COPY --from=builder /build/package*.json ./

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser`, runtime),
		WorkDir:    "/app",
		RunCommand: startCmd,
	}
}

// getPythonTemplate returns optimized Dockerfile for Python
func getPythonTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "3.12" // Default Python version
	}

	buildTool := string(analysis.BuildTool)
	startCmd := analysis.StartCommand
	if startCmd == "" {
		startCmd = "python app.py"
	}

	var installCmd string
	if buildTool == "poetry" {
		installCmd = `# Install poetry
RUN pip install --no-cache-dir poetry

# Copy poetry files
COPY pyproject.toml poetry.lock* ./

# Install dependencies
RUN poetry config virtualenvs.create false && \
    poetry install --no-dev --no-interaction --no-ansi`
	} else {
		installCmd = `# Copy requirements
COPY requirements.txt .

# Install dependencies
RUN pip install --no-cache-dir -r requirements.txt`
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("python:%s-slim", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM python:%s-slim AS builder
WORKDIR /build

%s

# Copy source code
COPY . .`, runtime, installCmd),
		RuntimeStage: fmt.Sprintf(`# Runtime stage
FROM python:%s-slim
WORKDIR /app

# Create non-root user
RUN useradd -m -u 1000 appuser

# Copy dependencies and code from builder
COPY --from=builder /usr/local/lib/python*/site-packages /usr/local/lib/python*/site-packages
COPY --from=builder /build .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser`, runtime),
		WorkDir:    "/app",
		RunCommand: startCmd,
	}
}

// getJavaTemplate returns optimized multi-stage Dockerfile for Java
func getJavaTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "21" // Default Java version
	}

	buildTool := string(analysis.BuildTool)
	var buildCmd string

	if buildTool == "gradle" {
		buildCmd = "./gradlew build -x test"
	} else { // maven
		buildCmd = "mvn clean package -DskipTests"
	}

	if analysis.BuildCommand != "" {
		buildCmd = analysis.BuildCommand
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("eclipse-temurin:%s-jdk-alpine", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM eclipse-temurin:%s-jdk-alpine AS builder
WORKDIR /build

# Copy build files
COPY pom.xml* build.gradle* settings.gradle* gradlew* gradle* ./
COPY gradle ./gradle
COPY mvnw* .mvn* ./

# Download dependencies (cached layer)
RUN if [ -f "pom.xml" ]; then mvn dependency:go-offline; fi
RUN if [ -f "build.gradle" ]; then ./gradlew dependencies; fi

# Copy source code
COPY src ./src

# Build application
RUN %s`, runtime, buildCmd),
		RuntimeStage: fmt.Sprintf(`# Runtime stage
FROM eclipse-temurin:%s-jre-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy JAR file
COPY --from=builder /build/target/*.jar app.jar

# Change ownership
RUN chown appuser:appuser app.jar

USER appuser`, runtime),
		WorkDir:    "/app",
		RunCommand: "java -jar app.jar",
	}
}

// getRustTemplate returns optimized multi-stage Dockerfile for Rust
func getRustTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	return &LanguageTemplate{
		BaseImage: "rust:1.75-alpine",
		BuildStage: `# Build stage
FROM rust:1.75-alpine AS builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache musl-dev

# Copy Cargo files
COPY Cargo.toml Cargo.lock* ./

# Copy source code
COPY src ./src

# Build application
RUN cargo build --release`,
		RuntimeStage: `# Runtime stage
FROM alpine:latest
WORKDIR /app

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy binary from builder
COPY --from=builder /build/target/release/app .

# Change ownership
RUN chown appuser:appuser app

USER appuser`,
		WorkDir:    "/app",
		RunCommand: "./app",
	}
}

// getRubyTemplate returns Dockerfile for Ruby
func getRubyTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "3.3" // Default Ruby version
	}

	startCmd := analysis.StartCommand
	if startCmd == "" {
		startCmd = "bundle exec ruby app.rb"
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("ruby:%s-alpine", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM ruby:%s-alpine AS builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache build-base

# Copy Gemfile
COPY Gemfile Gemfile.lock* ./

# Install gems
RUN bundle install --without development test

# Copy source code
COPY . .`, runtime),
		RuntimeStage: fmt.Sprintf(`# Runtime stage
FROM ruby:%s-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy gems and code from builder
COPY --from=builder /usr/local/bundle /usr/local/bundle
COPY --from=builder /build .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser`, runtime),
		WorkDir:    "/app",
		RunCommand: startCmd,
	}
}

// getPHPTemplate returns Dockerfile for PHP
func getPHPTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "8.3" // Default PHP version
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("php:%s-fpm-alpine", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM php:%s-fpm-alpine AS builder
WORKDIR /build

# Install composer
COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

# Copy composer files
COPY composer.json composer.lock* ./

# Install dependencies
RUN composer install --no-dev --optimize-autoloader

# Copy source code
COPY . .`, runtime),
		RuntimeStage: fmt.Sprintf(`# Runtime stage
FROM php:%s-fpm-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy application from builder
COPY --from=builder /build .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser`, runtime),
		WorkDir:    "/app",
		RunCommand: "php-fpm",
	}
}

// getDotNetTemplate returns optimized multi-stage Dockerfile for .NET
func getDotNetTemplate(analysis *analyzer.AnalysisResult) *LanguageTemplate {
	runtime := analysis.Runtime
	if runtime == "" {
		runtime = "8.0" // Default .NET version
	}

	return &LanguageTemplate{
		BaseImage: fmt.Sprintf("mcr.microsoft.com/dotnet/sdk:%s-alpine", runtime),
		BuildStage: fmt.Sprintf(`# Build stage
FROM mcr.microsoft.com/dotnet/sdk:%s-alpine AS builder
WORKDIR /build

# Copy csproj and restore
COPY *.csproj ./
RUN dotnet restore

# Copy source code
COPY . .

# Build and publish
RUN dotnet publish -c Release -o out`, runtime),
		RuntimeStage: fmt.Sprintf(`# Runtime stage
FROM mcr.microsoft.com/dotnet/aspnet:%s-alpine
WORKDIR /app

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Copy published app
COPY --from=builder /build/out .

# Change ownership
RUN chown -R appuser:appuser /app

USER appuser`, runtime),
		WorkDir:    "/app",
		RunCommand: "dotnet app.dll",
	}
}

// BuildDockerfileContent generates the complete Dockerfile content
func BuildDockerfileContent(template *LanguageTemplate, port int) string {
	var builder strings.Builder

	// Build stage
	builder.WriteString(template.BuildStage)
	builder.WriteString("\n\n")

	// Runtime stage
	builder.WriteString(template.RuntimeStage)
	builder.WriteString("\n\n")

	// Expose port
	if port > 0 {
		builder.WriteString(fmt.Sprintf("EXPOSE %d\n\n", port))
	}

	// CMD
	builder.WriteString(fmt.Sprintf("CMD [%s]\n", formatCmd(template.RunCommand)))

	return builder.String()
}

// formatCmd formats the run command for Dockerfile CMD instruction
func formatCmd(cmd string) string {
	parts := strings.Fields(cmd)
	quoted := make([]string, len(parts))
	for i, part := range parts {
		quoted[i] = fmt.Sprintf("\"%s\"", part)
	}
	return strings.Join(quoted, ", ")
}
