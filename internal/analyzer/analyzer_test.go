package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageDetector_DetectGo(t *testing.T) {
	detector := NewLanguageDetector()

	files := []FileInfo{
		{Name: "main.go", Extension: ".go"},
		{Name: "handler.go", Extension: ".go"},
		{Name: "go.mod", Extension: ".mod"},
	}

	language, confidence := detector.Detect(files)

	if language != LanguageGo {
		t.Errorf("Expected language Go, got %s", language)
	}

	if confidence < 0.9 {
		t.Errorf("Expected high confidence (>0.9), got %f", confidence)
	}
}

func TestLanguageDetector_DetectNodeJS(t *testing.T) {
	detector := NewLanguageDetector()

	files := []FileInfo{
		{Name: "package.json", Extension: ".json"},
		{Name: "index.js", Extension: ".js"},
		{Name: "app.ts", Extension: ".ts"},
	}

	language, confidence := detector.Detect(files)

	if language != LanguageNodeJS {
		t.Errorf("Expected language NodeJS, got %s", language)
	}

	if confidence < 0.9 {
		t.Errorf("Expected high confidence (>0.9), got %f", confidence)
	}
}

func TestLanguageDetector_DetectPython(t *testing.T) {
	detector := NewLanguageDetector()

	files := []FileInfo{
		{Name: "app.py", Extension: ".py"},
		{Name: "utils.py", Extension: ".py"},
		{Name: "requirements.txt", Extension: ".txt"},
	}

	language, confidence := detector.Detect(files)

	if language != LanguagePython {
		t.Errorf("Expected language Python, got %s", language)
	}

	if confidence < 0.9 {
		t.Errorf("Expected high confidence (>0.9), got %f", confidence)
	}
}

func TestFrameworkDetector_DetectExpressFromPackageJSON(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-express-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a package.json with Express
	packageJSON := `{
		"name": "test-app",
		"dependencies": {
			"express": "^4.18.0"
		}
	}`

	packagePath := filepath.Join(tempDir, "package.json")
	if err := os.WriteFile(packagePath, []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewFrameworkDetector()
	framework := detector.ParsePackageJSONFile(packagePath)

	if framework != FrameworkExpress {
		t.Errorf("Expected framework Express, got %s", framework)
	}
}

func TestFrameworkDetector_DetectFlaskFromRequirements(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-flask-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a requirements.txt with Flask
	requirements := "Flask==2.0.1\nrequests==2.26.0\n"

	reqPath := filepath.Join(tempDir, "requirements.txt")
	if err := os.WriteFile(reqPath, []byte(requirements), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewFrameworkDetector()
	framework := detector.ParseRequirementsFile(reqPath)

	if framework != FrameworkFlask {
		t.Errorf("Expected framework Flask, got %s", framework)
	}
}

func TestAnalyzer_ScanDirectory(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "test-scan-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create some test files
	testFiles := []string{
		"main.go",
		"handler.go",
		"README.md",
	}

	for _, file := range testFiles {
		path := filepath.Join(tempDir, file)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "pkg")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file in subdirectory
	subFile := filepath.Join(subDir, "util.go")
	if err := os.WriteFile(subFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	analyzer := New()
	files, err := analyzer.scanDirectory(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	// Should find at least 4 items (3 files + 1 directory + 1 file in subdir)
	if len(files) < 4 {
		t.Errorf("Expected at least 4 files, got %d", len(files))
	}
}

func TestDependencyParser_ParseNodeJS(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-nodejs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a package.json
	packageJSON := `{
		"name": "test-app",
		"version": "1.0.0",
		"scripts": {
			"start": "node index.js",
			"build": "webpack"
		},
		"dependencies": {
			"express": "^4.18.0"
		},
		"engines": {
			"node": "18.x"
		}
	}`

	packagePath := filepath.Join(tempDir, "package.json")
	if err := os.WriteFile(packagePath, []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewDependencyParser()
	buildInfo, err := parser.parseNodeJS(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if buildInfo.BuildTool != BuildToolNPM {
		t.Errorf("Expected build tool NPM, got %s", buildInfo.BuildTool)
	}

	if buildInfo.Runtime != "node:18.x" {
		t.Errorf("Expected runtime node:18.x, got %s", buildInfo.Runtime)
	}

	if _, exists := buildInfo.Dependencies["express"]; !exists {
		t.Error("Expected express in dependencies")
	}
}
