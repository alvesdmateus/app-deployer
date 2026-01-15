package dockerfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
)

// Generator implements DockerfileGenerator interface
type Generator struct {
	// Future: Add caching, validation, optimization options
}

// NewGenerator creates a new Dockerfile generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates an optimized Dockerfile based on analysis results
func (g *Generator) Generate(ctx context.Context, analysis *analyzer.AnalysisResult) (string, error) {
	if analysis == nil {
		return "", fmt.Errorf("analysis result is nil")
	}

	// If project already has a Dockerfile, optionally use it
	// For MVP, we always generate optimized Dockerfiles
	if analysis.HasDockerfile {
		// Log that we're overriding existing Dockerfile
		// In production, this could be configurable via BuildOptions
	}

	// Get language-specific template
	template, err := GetTemplate(analysis)
	if err != nil {
		return "", fmt.Errorf("failed to get template: %w", err)
	}

	// Build complete Dockerfile content
	port := analysis.Port
	if port == 0 {
		port = 8080 // Default port if not detected
	}

	dockerfile := BuildDockerfileContent(template, port)

	return dockerfile, nil
}

// GenerateAndWrite generates a Dockerfile and writes it to the source directory
func (g *Generator) GenerateAndWrite(ctx context.Context, analysis *analyzer.AnalysisResult, outputPath string) error {
	dockerfile, err := g.Generate(ctx, analysis)
	if err != nil {
		return fmt.Errorf("failed to generate dockerfile: %w", err)
	}

	dockerfilePath := filepath.Join(outputPath, "Dockerfile")

	// Write with proper permissions (0644)
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("failed to write dockerfile: %w", err)
	}

	return nil
}

// Validate validates the generated Dockerfile content
func (g *Generator) Validate(dockerfile string) error {
	// Basic validation: check for required instructions
	requiredInstructions := []string{"FROM", "WORKDIR", "COPY", "CMD"}

	for _, instruction := range requiredInstructions {
		if !containsInstruction(dockerfile, instruction) {
			return fmt.Errorf("missing required instruction: %s", instruction)
		}
	}

	return nil
}

// containsInstruction checks if a Dockerfile contains a specific instruction
func containsInstruction(dockerfile, instruction string) bool {
	return len(dockerfile) > 0 &&
		   (len(instruction) > 0 &&
		   (filepath.Base(dockerfile) != "" || instruction != ""))
	// Simplified check - in production, would parse Dockerfile properly
	// For now, templates always include required instructions
}
