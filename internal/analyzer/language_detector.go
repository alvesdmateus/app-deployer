package analyzer

import (
	"strings"
)

// LanguageDetector detects programming languages from source files
type LanguageDetector struct {
	// Language indicators map file extensions to languages
	extensionMap map[string]Language

	// Key files that indicate a language
	keyFiles map[string]Language
}

// NewLanguageDetector creates a new language detector
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{
		extensionMap: map[string]Language{
			".go":   LanguageGo,
			".js":   LanguageNodeJS,
			".ts":   LanguageNodeJS,
			".jsx":  LanguageNodeJS,
			".tsx":  LanguageNodeJS,
			".mjs":  LanguageNodeJS,
			".py":   LanguagePython,
			".java": LanguageJava,
			".kt":   LanguageJava,
			".rs":   LanguageRust,
			".rb":   LanguageRuby,
			".php":  LanguagePHP,
			".cs":   LanguageDotNet,
		},
		keyFiles: map[string]Language{
			"package.json":    LanguageNodeJS,
			"go.mod":          LanguageGo,
			"requirements.txt": LanguagePython,
			"pyproject.toml":  LanguagePython,
			"Pipfile":         LanguagePython,
			"pom.xml":         LanguageJava,
			"build.gradle":    LanguageJava,
			"Cargo.toml":      LanguageRust,
			"Gemfile":         LanguageRuby,
			"composer.json":   LanguagePHP,
		},
	}
}

// Detect detects the primary language from a list of files
func (ld *LanguageDetector) Detect(files []FileInfo) (Language, float64) {
	// Count files by language
	languageCounts := make(map[Language]int)
	totalFiles := 0

	// First check for key files (high confidence indicators)
	for _, file := range files {
		if lang, exists := ld.keyFiles[strings.ToLower(file.Name)]; exists {
			// Key file found - very high confidence
			return lang, 0.95
		}
	}

	// Count source files by extension
	for _, file := range files {
		if file.IsDirectory {
			continue
		}

		if lang, exists := ld.extensionMap[file.Extension]; exists {
			languageCounts[lang]++
			totalFiles++
		}
	}

	if totalFiles == 0 {
		return LanguageUnknown, 0.0
	}

	// Find the most common language
	var primaryLanguage Language
	maxCount := 0

	for lang, count := range languageCounts {
		if count > maxCount {
			maxCount = count
			primaryLanguage = lang
		}
	}

	if primaryLanguage == "" {
		return LanguageUnknown, 0.0
	}

	// Calculate confidence based on percentage of files
	confidence := float64(maxCount) / float64(totalFiles)

	return primaryLanguage, confidence
}
