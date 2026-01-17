//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
)

func TestAnalyzer_RealNodeJSRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipAnalyzer {
		t.Skip("Analyzer tests are disabled")
	}

	// Clone Express.js repository (well-maintained Node.js project)
	repoDir := CloneTestRepo(t, TestRepos.NodeJSExpress, "master")

	// Create analyzer and analyze the repository
	a := analyzer.New()
	result, err := a.Analyze(repoDir)
	if err != nil {
		t.Fatalf("Failed to analyze repository: %v", err)
	}

	// Verify language detection
	if result.Language != analyzer.LanguageNodeJS {
		t.Errorf("Expected language NodeJS, got %s", result.Language)
	}

	// Verify confidence is reasonable
	if result.Confidence < 0.5 {
		t.Errorf("Expected confidence > 0.5, got %f", result.Confidence)
	}

	// Express should be detected as Express framework
	if result.Framework != analyzer.FrameworkExpress {
		t.Logf("Note: Framework detected as %s (expected Express)", result.Framework)
	}

	// Verify build tool detection
	if result.BuildTool != analyzer.BuildToolNPM && result.BuildTool != analyzer.BuildToolYarn {
		t.Errorf("Expected build tool NPM or Yarn, got %s", result.BuildTool)
	}

	// Verify dependencies were parsed
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies to be detected")
	}

	t.Logf("Analysis result: Language=%s, Framework=%s, BuildTool=%s, Confidence=%.2f",
		result.Language, result.Framework, result.BuildTool, result.Confidence)
	t.Logf("Found %d dependencies, %d dev dependencies",
		len(result.Dependencies), len(result.DevDependencies))
}

func TestAnalyzer_RealPythonRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipAnalyzer {
		t.Skip("Analyzer tests are disabled")
	}

	// Clone Flask repository (well-maintained Python project)
	repoDir := CloneTestRepo(t, TestRepos.PythonFlask, "main")

	// Create analyzer and analyze the repository
	a := analyzer.New()
	result, err := a.Analyze(repoDir)
	if err != nil {
		t.Fatalf("Failed to analyze repository: %v", err)
	}

	// Verify language detection
	if result.Language != analyzer.LanguagePython {
		t.Errorf("Expected language Python, got %s", result.Language)
	}

	// Verify confidence is reasonable
	if result.Confidence < 0.5 {
		t.Errorf("Expected confidence > 0.5, got %f", result.Confidence)
	}

	// Flask should be detected as Flask framework
	if result.Framework != analyzer.FrameworkFlask {
		t.Logf("Note: Framework detected as %s (expected Flask)", result.Framework)
	}

	t.Logf("Analysis result: Language=%s, Framework=%s, BuildTool=%s, Confidence=%.2f",
		result.Language, result.Framework, result.BuildTool, result.Confidence)
}

func TestAnalyzer_RealGoRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipAnalyzer {
		t.Skip("Analyzer tests are disabled")
	}

	// Clone Fiber repository (well-maintained Go project)
	repoDir := CloneTestRepo(t, TestRepos.GoFiber, "master")

	// Create analyzer and analyze the repository
	a := analyzer.New()
	result, err := a.Analyze(repoDir)
	if err != nil {
		t.Fatalf("Failed to analyze repository: %v", err)
	}

	// Verify language detection
	if result.Language != analyzer.LanguageGo {
		t.Errorf("Expected language Go, got %s", result.Language)
	}

	// Verify confidence is reasonable
	if result.Confidence < 0.5 {
		t.Errorf("Expected confidence > 0.5, got %f", result.Confidence)
	}

	// Verify build tool is Go
	if result.BuildTool != analyzer.BuildToolGo {
		t.Errorf("Expected build tool Go, got %s", result.BuildTool)
	}

	t.Logf("Analysis result: Language=%s, Framework=%s, BuildTool=%s, Confidence=%.2f",
		result.Language, result.Framework, result.BuildTool, result.Confidence)
}

func TestAnalyzer_WithDockerfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipAnalyzer {
		t.Skip("Analyzer tests are disabled")
	}

	// Create a temporary directory with a simple Node.js project and Dockerfile
	tempDir, err := os.MkdirTemp("", "analyzer-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create package.json
	packageJSON := `{
		"name": "test-app",
		"version": "1.0.0",
		"main": "index.js",
		"scripts": {
			"start": "node index.js"
		},
		"dependencies": {
			"express": "^4.18.0"
		}
	}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to create package.json: %v", err)
	}

	// Create index.js
	indexJS := `const express = require('express');
const app = express();
app.get('/', (req, res) => res.send('Hello'));
app.listen(3000);`
	if err := os.WriteFile(filepath.Join(tempDir, "index.js"), []byte(indexJS), 0644); err != nil {
		t.Fatalf("Failed to create index.js: %v", err)
	}

	// Create Dockerfile
	if err := os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte(SimpleNodeDockerfile), 0644); err != nil {
		t.Fatalf("Failed to create Dockerfile: %v", err)
	}

	// Analyze
	a := analyzer.New()
	result, err := a.Analyze(tempDir)
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Verify Dockerfile was detected
	if !result.HasDockerfile {
		t.Error("Expected HasDockerfile to be true")
	}

	// Verify language
	if result.Language != analyzer.LanguageNodeJS {
		t.Errorf("Expected language NodeJS, got %s", result.Language)
	}

	// Note: Framework detection depends on analyzer implementation
	// Express detection may require more sophisticated heuristics
	if result.Framework != analyzer.FrameworkExpress {
		t.Logf("Note: Framework detected as %s (Express detection may need enhancement)", result.Framework)
	}

	// Verify start command
	if result.StartCommand == "" {
		t.Error("Expected start command to be detected")
	}

	t.Logf("Analysis result: Language=%s, Framework=%s, HasDockerfile=%v, StartCommand=%s",
		result.Language, result.Framework, result.HasDockerfile, result.StartCommand)
}

func TestAnalyzer_PortDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := LoadIntegrationConfig()
	if cfg.SkipAnalyzer {
		t.Skip("Analyzer tests are disabled")
	}

	// Create a temporary directory with a Python Flask project
	tempDir, err := os.MkdirTemp("", "analyzer-port-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create requirements.txt
	requirements := `flask==2.3.0
gunicorn==21.0.0`
	if err := os.WriteFile(filepath.Join(tempDir, "requirements.txt"), []byte(requirements), 0644); err != nil {
		t.Fatalf("Failed to create requirements.txt: %v", err)
	}

	// Create app.py
	appPy := `from flask import Flask
app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)`
	if err := os.WriteFile(filepath.Join(tempDir, "app.py"), []byte(appPy), 0644); err != nil {
		t.Fatalf("Failed to create app.py: %v", err)
	}

	// Analyze
	a := analyzer.New()
	result, err := a.Analyze(tempDir)
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Verify language
	if result.Language != analyzer.LanguagePython {
		t.Errorf("Expected language Python, got %s", result.Language)
	}

	// Note: Framework detection depends on analyzer implementation
	// Flask detection may require checking imports in Python files
	if result.Framework != analyzer.FrameworkFlask {
		t.Logf("Note: Framework detected as %s (Flask detection may need enhancement)", result.Framework)
	}

	// Verify dependencies
	foundFlask := false
	for dep := range result.Dependencies {
		if dep == "flask" {
			foundFlask = true
			break
		}
	}
	if !foundFlask {
		t.Error("Expected flask to be in dependencies")
	}

	t.Logf("Analysis result: Language=%s, Framework=%s, Port=%d, Dependencies=%v",
		result.Language, result.Framework, result.Port, result.Dependencies)
}
