package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// Analyzer analyzes source code to detect language, framework, and dependencies
type Analyzer struct {
	languageDetector  *LanguageDetector
	frameworkDetector *FrameworkDetector
	dependencyParser  *DependencyParser
}

// New creates a new analyzer
func New() *Analyzer {
	return &Analyzer{
		languageDetector:  NewLanguageDetector(),
		frameworkDetector: NewFrameworkDetector(),
		dependencyParser:  NewDependencyParser(),
	}
}

// Analyze analyzes a directory of source code
func (a *Analyzer) Analyze(path string) (*AnalysisResult, error) {
	log.Info().Str("path", path).Msg("Starting source code analysis")

	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", path)
	}

	// Scan directory for files
	files, err := a.scanDirectory(path)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in directory")
	}

	result := &AnalysisResult{
		Files: make([]string, 0, len(files)),
	}

	// Collect file names
	for _, file := range files {
		result.Files = append(result.Files, file.Name)
	}

	// Detect language
	language, confidence := a.languageDetector.Detect(files)
	result.Language = language
	result.Confidence = confidence

	log.Info().
		Str("language", string(language)).
		Float64("confidence", confidence).
		Msg("Language detected")

	// Detect framework
	framework := a.frameworkDetector.Detect(language, files)
	result.Framework = framework

	// Parse dependencies and build configuration
	buildInfo, err := a.dependencyParser.Parse(path, language)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to parse dependencies")
	} else {
		result.BuildTool = buildInfo.BuildTool
		result.Runtime = buildInfo.Runtime
		result.Dependencies = buildInfo.Dependencies
		result.DevDependencies = buildInfo.DevDependencies
		result.StartCommand = buildInfo.StartCommand
		result.BuildCommand = buildInfo.BuildCommand
		result.Port = buildInfo.Port
	}

	// Check for Dockerfile
	result.HasDockerfile = a.hasDockerfile(files)

	log.Info().
		Str("language", string(result.Language)).
		Str("framework", string(result.Framework)).
		Str("build_tool", string(result.BuildTool)).
		Bool("has_dockerfile", result.HasDockerfile).
		Msg("Analysis complete")

	return result, nil
}

// scanDirectory scans a directory and returns file information
func (a *Analyzer) scanDirectory(path string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common directories to ignore
		if info.IsDir() {
			ignored := []string{"node_modules", "vendor", "venv", ".git", "dist", "build", "target", "__pycache__"}
			for _, dir := range ignored {
				if info.Name() == dir {
					return filepath.SkipDir
				}
			}
		}

		relPath, _ := filepath.Rel(path, filePath)

		fileInfo := FileInfo{
			Path:        relPath,
			Name:        info.Name(),
			Extension:   strings.ToLower(filepath.Ext(info.Name())),
			IsDirectory: info.IsDir(),
			Size:        info.Size(),
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// hasDockerfile checks if a Dockerfile exists
func (a *Analyzer) hasDockerfile(files []FileInfo) bool {
	for _, file := range files {
		if strings.EqualFold(file.Name, "Dockerfile") || strings.HasPrefix(strings.ToLower(file.Name), "dockerfile") {
			return true
		}
	}
	return false
}
