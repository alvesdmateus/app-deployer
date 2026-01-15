package dockerfile

import (
	"strings"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
)

// OptimizationOptions contains options for Dockerfile optimization
type OptimizationOptions struct {
	EnableMultiStage   bool // Use multi-stage builds
	EnableLayerCaching bool // Optimize for layer caching
	MinimalBaseImage   bool // Use minimal base images (alpine, distroless)
	SecurityHardening  bool // Apply security best practices
}

// DefaultOptimizationOptions returns default optimization settings
func DefaultOptimizationOptions() OptimizationOptions {
	return OptimizationOptions{
		EnableMultiStage:   true,
		EnableLayerCaching: true,
		MinimalBaseImage:   true,
		SecurityHardening:  true,
	}
}

// Optimizer optimizes Dockerfile generation
type Optimizer struct {
	options OptimizationOptions
}

// NewOptimizer creates a new Dockerfile optimizer
func NewOptimizer(options OptimizationOptions) *Optimizer {
	return &Optimizer{
		options: options,
	}
}

// OptimizeForLanguage applies language-specific optimizations
func (o *Optimizer) OptimizeForLanguage(analysis *analyzer.AnalysisResult) []string {
	var optimizations []string

	switch analysis.Language {
	case analyzer.LanguageGo:
		optimizations = append(optimizations, o.optimizeGo()...)
	case analyzer.LanguageNodeJS:
		optimizations = append(optimizations, o.optimizeNode()...)
	case analyzer.LanguagePython:
		optimizations = append(optimizations, o.optimizePython()...)
	case analyzer.LanguageJava:
		optimizations = append(optimizations, o.optimizeJava()...)
	}

	// Common optimizations for all languages
	if o.options.SecurityHardening {
		optimizations = append(optimizations, o.applySecurityHardening()...)
	}

	return optimizations
}

// optimizeGo returns Go-specific optimizations
func (o *Optimizer) optimizeGo() []string {
	return []string{
		"CGO_ENABLED=0 for static binary",
		"GOOS=linux for Linux containers",
		"Multi-stage build with alpine runtime",
		"Stripped binary for smaller size",
	}
}

// optimizeNode returns Node.js-specific optimizations
func (o *Optimizer) optimizeNode() []string {
	return []string{
		"Use npm ci instead of npm install for faster, reproducible builds",
		"Copy package files before source code for better caching",
		"NODE_ENV=production for production builds",
		"Remove dev dependencies in production stage",
	}
}

// optimizePython returns Python-specific optimizations
func (o *Optimizer) optimizePython() []string {
	return []string{
		"Use slim base images",
		"pip install --no-cache-dir to reduce image size",
		"Multi-stage build to separate build dependencies",
		"Copy only necessary files from build stage",
	}
}

// optimizeJava returns Java-specific optimizations
func (o *Optimizer) optimizeJava() []string {
	return []string{
		"Use JRE instead of JDK for runtime",
		"Multi-stage build to separate build tools",
		"Cache dependency downloads in separate layer",
		"Use alpine-based images for smaller size",
	}
}

// applySecurityHardening returns security best practices
func (o *Optimizer) applySecurityHardening() []string {
	return []string{
		"Run as non-root user",
		"Use specific version tags, not 'latest'",
		"Minimal base images (alpine, distroless)",
		"Read-only root filesystem where possible",
		"Drop unnecessary capabilities",
	}
}

// GetImageSizeEstimate estimates the final image size
func (o *Optimizer) GetImageSizeEstimate(analysis *analyzer.AnalysisResult) string {
	// Rough estimates based on language and base image
	estimates := map[analyzer.Language]string{
		analyzer.LanguageGo:     "10-30 MB",
		analyzer.LanguageNodeJS: "150-300 MB",
		analyzer.LanguagePython: "100-200 MB",
		analyzer.LanguageJava:   "200-400 MB",
		analyzer.LanguageRust:   "10-50 MB",
		analyzer.LanguageRuby:   "150-250 MB",
		analyzer.LanguagePHP:    "100-200 MB",
		analyzer.LanguageDotNet: "150-300 MB",
	}

	if estimate, ok := estimates[analysis.Language]; ok {
		return estimate
	}

	return "Unknown"
}

// SuggestImprovements analyzes a Dockerfile and suggests improvements
func (o *Optimizer) SuggestImprovements(dockerfile string) []string {
	var suggestions []string

	// Check for common anti-patterns
	if !strings.Contains(dockerfile, "USER") {
		suggestions = append(suggestions, "Add non-root user for better security")
	}

	if strings.Contains(dockerfile, "FROM latest") {
		suggestions = append(suggestions, "Use specific version tags instead of 'latest'")
	}

	if !strings.Contains(dockerfile, "AS builder") && o.options.EnableMultiStage {
		suggestions = append(suggestions, "Consider using multi-stage build to reduce image size")
	}

	if strings.Contains(dockerfile, "apt-get install") && !strings.Contains(dockerfile, "rm -rf /var/lib/apt/lists") {
		suggestions = append(suggestions, "Clean apt cache to reduce image size")
	}

	return suggestions
}
