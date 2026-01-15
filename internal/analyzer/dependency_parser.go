package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// BuildInfo contains build and runtime information
type BuildInfo struct {
	BuildTool       BuildTool
	Runtime         string
	Dependencies    map[string]string
	DevDependencies map[string]string
	StartCommand    string
	BuildCommand    string
	Port            int
}

// DependencyParser parses dependency files
type DependencyParser struct{}

// NewDependencyParser creates a new dependency parser
func NewDependencyParser() *DependencyParser {
	return &DependencyParser{}
}

// Parse parses dependency files based on language
func (dp *DependencyParser) Parse(basePath string, language Language) (*BuildInfo, error) {
	switch language {
	case LanguageNodeJS:
		return dp.parseNodeJS(basePath)
	case LanguageGo:
		return dp.parseGo(basePath)
	case LanguagePython:
		return dp.parsePython(basePath)
	case LanguageJava:
		return dp.parseJava(basePath)
	default:
		return &BuildInfo{
			BuildTool: BuildToolUnknown,
		}, nil
	}
}

// parseNodeJS parses Node.js project files
func (dp *DependencyParser) parseNodeJS(basePath string) (*BuildInfo, error) {
	info := &BuildInfo{
		BuildTool:       BuildToolNPM,
		Runtime:         "node",
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
		Port:            3000,
	}

	// Check for package.json
	packageJSONPath := filepath.Join(basePath, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read package.json")
		return info, nil
	}

	var pkg struct {
		Name            string            `json:"name"`
		Version         string            `json:"version"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Engines         struct {
			Node string `json:"node"`
			NPM  string `json:"npm"`
		} `json:"engines"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		log.Warn().Err(err).Msg("Failed to parse package.json")
		return info, nil
	}

	// Copy dependencies
	info.Dependencies = pkg.Dependencies
	info.DevDependencies = pkg.DevDependencies

	// Detect build tool
	if _, err := os.Stat(filepath.Join(basePath, "yarn.lock")); err == nil {
		info.BuildTool = BuildToolYarn
		info.BuildCommand = "yarn install && yarn build"
		info.StartCommand = "yarn start"
	} else if _, err := os.Stat(filepath.Join(basePath, "pnpm-lock.yaml")); err == nil {
		info.BuildTool = BuildToolPNPM
		info.BuildCommand = "pnpm install && pnpm build"
		info.StartCommand = "pnpm start"
	} else {
		info.BuildCommand = "npm install && npm run build"
		info.StartCommand = "npm start"
	}

	// Check for custom scripts
	if startScript, exists := pkg.Scripts["start"]; exists && startScript != "" {
		// Start command already set based on build tool
		log.Debug().Str("start_script", startScript).Msg("Found start script")
	}

	if buildScript, exists := pkg.Scripts["build"]; exists && buildScript != "" {
		log.Debug().Str("build_script", buildScript).Msg("Found build script")
	}

	// Set runtime version if specified
	if pkg.Engines.Node != "" {
		info.Runtime = "node:" + pkg.Engines.Node
	}

	return info, nil
}

// parseGo parses Go project files
func (dp *DependencyParser) parseGo(basePath string) (*BuildInfo, error) {
	info := &BuildInfo{
		BuildTool:    BuildToolGo,
		Runtime:      "go",
		Dependencies: make(map[string]string),
		BuildCommand: "go build -o app .",
		StartCommand: "./app",
		Port:         8080,
	}

	// Check for go.mod
	goModPath := filepath.Join(basePath, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read go.mod")
		return info, nil
	}

	// Parse go.mod for version
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			version := strings.TrimPrefix(line, "go ")
			info.Runtime = "go:" + version
			break
		}
	}

	return info, nil
}

// parsePython parses Python project files
func (dp *DependencyParser) parsePython(basePath string) (*BuildInfo, error) {
	info := &BuildInfo{
		BuildTool:    BuildToolPip,
		Runtime:      "python:3.11",
		Dependencies: make(map[string]string),
		Port:         5000,
	}

	// Check for requirements.txt
	requirementsPath := filepath.Join(basePath, "requirements.txt")
	if data, err := os.ReadFile(requirementsPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			// Parse package==version format
			parts := strings.Split(line, "==")
			if len(parts) == 2 {
				info.Dependencies[parts[0]] = parts[1]
			} else {
				info.Dependencies[line] = "*"
			}
		}

		info.BuildCommand = "pip install -r requirements.txt"
	}

	// Check for pyproject.toml (Poetry)
	pyprojectPath := filepath.Join(basePath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		info.BuildTool = BuildToolPoetry
		info.BuildCommand = "poetry install"
		info.StartCommand = "poetry run python app.py"
	} else {
		// Try to detect main file
		mainFiles := []string{"app.py", "main.py", "manage.py"}
		for _, mainFile := range mainFiles {
			if _, err := os.Stat(filepath.Join(basePath, mainFile)); err == nil {
				info.StartCommand = "python " + mainFile
				break
			}
		}
	}

	return info, nil
}

// parseJava parses Java project files
func (dp *DependencyParser) parseJava(basePath string) (*BuildInfo, error) {
	info := &BuildInfo{
		Runtime:      "java:17",
		Dependencies: make(map[string]string),
		Port:         8080,
	}

	// Check for Maven
	pomPath := filepath.Join(basePath, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		info.BuildTool = BuildToolMaven
		info.BuildCommand = "mvn clean package"
		info.StartCommand = "java -jar target/*.jar"
		return info, nil
	}

	// Check for Gradle
	gradlePaths := []string{"build.gradle", "build.gradle.kts"}
	for _, gradleFile := range gradlePaths {
		if _, err := os.Stat(filepath.Join(basePath, gradleFile)); err == nil {
			info.BuildTool = BuildToolGradle
			info.BuildCommand = "./gradlew build"
			info.StartCommand = "java -jar build/libs/*.jar"
			return info, nil
		}
	}

	info.BuildTool = BuildToolUnknown
	return info, nil
}
