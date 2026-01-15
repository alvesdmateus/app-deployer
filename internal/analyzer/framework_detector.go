package analyzer

import (
	"encoding/json"
	"os"
	"strings"
)

// FrameworkDetector detects web frameworks
type FrameworkDetector struct{}

// NewFrameworkDetector creates a new framework detector
func NewFrameworkDetector() *FrameworkDetector {
	return &FrameworkDetector{}
}

// Detect detects the framework based on language and files
func (fd *FrameworkDetector) Detect(language Language, files []FileInfo) Framework {
	switch language {
	case LanguageGo:
		return fd.detectGoFramework(files)
	case LanguageNodeJS:
		return fd.detectNodeFramework(files)
	case LanguagePython:
		return fd.detectPythonFramework(files)
	case LanguageJava:
		return fd.detectJavaFramework(files)
	default:
		return FrameworkUnknown
	}
}

// detectGoFramework detects Go frameworks
func (fd *FrameworkDetector) detectGoFramework(files []FileInfo) Framework {
	// Look for go.mod file and check imports
	for _, file := range files {
		if file.Name == "go.mod" {
			// In a real implementation, we would parse go.mod
			// For now, we'll check for common framework patterns in file names
			break
		}
	}

	// Check source files for framework imports
	patterns := map[string]Framework{
		"gin":   FrameworkGin,
		"echo":  FrameworkEcho,
		"chi":   FrameworkChi,
		"fiber": FrameworkFiber,
	}

	for _, file := range files {
		if file.Extension == ".go" {
			lowerPath := strings.ToLower(file.Path)
			for pattern, framework := range patterns {
				if strings.Contains(lowerPath, pattern) {
					return framework
				}
			}
		}
	}

	return FrameworkUnknown
}

// detectNodeFramework detects Node.js frameworks
func (fd *FrameworkDetector) detectNodeFramework(files []FileInfo) Framework {
	// Find and parse package.json
	for _, file := range files {
		if file.Name == "package.json" {
			// In a real scenario, we'd have the base path
			// For now, check for common patterns
			return fd.parsePackageJSON(file.Path)
		}
	}

	// Check for Next.js specific files
	for _, file := range files {
		if file.Name == "next.config.js" || file.Name == "next.config.mjs" {
			return FrameworkNextJS
		}
	}

	return FrameworkUnknown
}

// parsePackageJSON parses package.json to detect framework
func (fd *FrameworkDetector) parsePackageJSON(path string) Framework {
	// This is a placeholder - in real implementation we'd read and parse the file
	// For now, return unknown as we don't have the full path context
	return FrameworkUnknown
}

// ParsePackageJSONFile parses a package.json file from filesystem
func (fd *FrameworkDetector) ParsePackageJSONFile(filePath string) Framework {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return FrameworkUnknown
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return FrameworkUnknown
	}

	// Check dependencies for framework indicators
	frameworks := map[string]Framework{
		"express":   FrameworkExpress,
		"@nestjs/core": FrameworkNestJS,
		"next":      FrameworkNextJS,
		"koa":       FrameworkKoa,
		"fastify":   FrameworkFastify,
	}

	// Check both dependencies and devDependencies
	allDeps := make(map[string]string)
	for k, v := range pkg.Dependencies {
		allDeps[k] = v
	}
	for k, v := range pkg.DevDependencies {
		allDeps[k] = v
	}

	for dep, framework := range frameworks {
		if _, exists := allDeps[dep]; exists {
			return framework
		}
	}

	return FrameworkUnknown
}

// detectPythonFramework detects Python frameworks
func (fd *FrameworkDetector) detectPythonFramework(files []FileInfo) Framework {
	// Look for requirements.txt or pyproject.toml
	for _, file := range files {
		if file.Name == "requirements.txt" {
			// Would parse requirements.txt
			return FrameworkUnknown
		}
		if file.Name == "pyproject.toml" {
			// Would parse pyproject.toml
			return FrameworkUnknown
		}
	}

	// Check for common framework patterns in file structure
	for _, file := range files {
		lowerPath := strings.ToLower(file.Path)
		if strings.Contains(lowerPath, "django") {
			return FrameworkDjango
		}
		if strings.Contains(lowerPath, "flask") {
			return FrameworkFlask
		}
		if strings.Contains(lowerPath, "fastapi") {
			return FrameworkFastAPI
		}
	}

	return FrameworkUnknown
}

// ParseRequirementsFile parses a requirements.txt file
func (fd *FrameworkDetector) ParseRequirementsFile(filePath string) Framework {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return FrameworkUnknown
	}

	content := strings.ToLower(string(data))
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "django") {
			return FrameworkDjango
		}
		if strings.HasPrefix(line, "flask") {
			return FrameworkFlask
		}
		if strings.HasPrefix(line, "fastapi") {
			return FrameworkFastAPI
		}
	}

	return FrameworkUnknown
}

// detectJavaFramework detects Java frameworks
func (fd *FrameworkDetector) detectJavaFramework(files []FileInfo) Framework {
	// Look for pom.xml or build.gradle
	for _, file := range files {
		if file.Name == "pom.xml" {
			// Would parse pom.xml for Spring Boot
			return FrameworkSpringBoot
		}
		if file.Name == "build.gradle" || file.Name == "build.gradle.kts" {
			// Would parse Gradle file
			return FrameworkSpringBoot
		}
	}

	// Check for application.properties or application.yml (Spring Boot indicators)
	for _, file := range files {
		if file.Name == "application.properties" || file.Name == "application.yml" {
			return FrameworkSpringBoot
		}
	}

	return FrameworkUnknown
}

// GetFrameworkInfo returns additional information about a framework
func GetFrameworkInfo(framework Framework) map[string]interface{} {
	info := map[Framework]map[string]interface{}{
		FrameworkExpress: {
			"name":          "Express.js",
			"language":      "nodejs",
			"default_port":  3000,
			"build_command": "npm install",
			"start_command": "npm start",
		},
		FrameworkNextJS: {
			"name":          "Next.js",
			"language":      "nodejs",
			"default_port":  3000,
			"build_command": "npm run build",
			"start_command": "npm start",
		},
		FrameworkGin: {
			"name":          "Gin",
			"language":      "go",
			"default_port":  8080,
			"build_command": "go build",
			"start_command": "./app",
		},
		FrameworkFlask: {
			"name":          "Flask",
			"language":      "python",
			"default_port":  5000,
			"build_command": "pip install -r requirements.txt",
			"start_command": "python app.py",
		},
		FrameworkDjango: {
			"name":          "Django",
			"language":      "python",
			"default_port":  8000,
			"build_command": "pip install -r requirements.txt",
			"start_command": "python manage.py runserver",
		},
		FrameworkSpringBoot: {
			"name":          "Spring Boot",
			"language":      "java",
			"default_port":  8080,
			"build_command": "mvn clean install",
			"start_command": "java -jar target/app.jar",
		},
	}

	if frameworkInfo, exists := info[framework]; exists {
		return frameworkInfo
	}

	return map[string]interface{}{
		"name":     string(framework),
		"language": "unknown",
	}
}
