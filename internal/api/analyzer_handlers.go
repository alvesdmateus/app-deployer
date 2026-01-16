package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/alvesdmateus/app-deployer/internal/analyzer"
	"github.com/rs/zerolog/log"
)

// AnalyzerHandler handles source code analysis requests
type AnalyzerHandler struct {
	analyzer *analyzer.Analyzer
}

// NewAnalyzerHandler creates a new analyzer handler
func NewAnalyzerHandler() *AnalyzerHandler {
	return &AnalyzerHandler{
		analyzer: analyzer.New(),
	}
}

// AnalyzeRequest represents a request to analyze source code
type AnalyzeRequest struct {
	Path string `json:"path"`
}

// AnalyzeSourceCode handles POST /api/v1/analyze
func (h *AnalyzerHandler) AnalyzeSourceCode(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Path == "" {
		RespondWithError(w, http.StatusBadRequest, "Path is required")
		return
	}

	// Analyze the source code
	result, err := h.analyzer.Analyze(req.Path)
	if err != nil {
		log.Error().Err(err).Str("path", req.Path).Msg("Failed to analyze source code")
		RespondWithError(w, http.StatusInternalServerError, "Failed to analyze source code: "+err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, result)
}

// UploadAndAnalyze handles POST /api/v1/analyze/upload
func (h *AnalyzerHandler) UploadAndAnalyze(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32 MB max
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("source")
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "No source file provided")
		return
	}
	defer file.Close()

	// Create temporary directory for upload
	tempDir, err := os.MkdirTemp("", "app-deployer-*")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create temp directory")
		RespondWithError(w, http.StatusInternalServerError, "Failed to create temp directory")
		return
	}
	defer os.RemoveAll(tempDir)

	// Save uploaded file
	destPath := filepath.Join(tempDir, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create destination file")
		RespondWithError(w, http.StatusInternalServerError, "Failed to save uploaded file")
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		log.Error().Err(err).Msg("Failed to save file")
		RespondWithError(w, http.StatusInternalServerError, "Failed to save uploaded file")
		return
	}

	// Check if uploaded file is a zip/tar and extract it
	// For now, we'll assume it's a directory structure
	// In production, you'd want to handle zip/tar.gz extraction

	// Analyze the uploaded code
	result, err := h.analyzer.Analyze(tempDir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to analyze uploaded source code")
		RespondWithError(w, http.StatusInternalServerError, "Failed to analyze source code: "+err.Error())
		return
	}

	RespondWithJSON(w, http.StatusOK, result)
}

// GetSupportedLanguages handles GET /api/v1/analyze/languages
func (h *AnalyzerHandler) GetSupportedLanguages(w http.ResponseWriter, r *http.Request) {
	languages := []map[string]interface{}{
		{
			"id":         "go",
			"name":       "Go",
			"frameworks": []string{"gin", "echo", "chi", "fiber"},
		},
		{
			"id":         "nodejs",
			"name":       "Node.js",
			"frameworks": []string{"express", "nestjs", "nextjs", "koa", "fastify"},
		},
		{
			"id":         "python",
			"name":       "Python",
			"frameworks": []string{"flask", "django", "fastapi"},
		},
		{
			"id":         "java",
			"name":       "Java",
			"frameworks": []string{"springboot", "quarkus"},
		},
		{
			"id":         "rust",
			"name":       "Rust",
			"frameworks": []string{},
		},
		{
			"id":         "ruby",
			"name":       "Ruby",
			"frameworks": []string{},
		},
		{
			"id":         "php",
			"name":       "PHP",
			"frameworks": []string{},
		},
		{
			"id":         "dotnet",
			"name":       ".NET",
			"frameworks": []string{},
		},
	}

	RespondWithJSON(w, http.StatusOK, map[string]interface{}{
		"languages": languages,
	})
}
