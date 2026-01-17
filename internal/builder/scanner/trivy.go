package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ScanConfig holds configuration for vulnerability scanning
type ScanConfig struct {
	Enabled            bool
	FailOnCritical     bool
	FailOnHigh         bool
	FailOnMedium       bool
	IgnoreUnfixed      bool
	Timeout            time.Duration
	SeverityThresholds SeverityThresholds
}

// SeverityThresholds defines the maximum allowed vulnerabilities per severity
type SeverityThresholds struct {
	Critical int // Max critical vulnerabilities allowed (-1 for unlimited)
	High     int // Max high vulnerabilities allowed (-1 for unlimited)
	Medium   int // Max medium vulnerabilities allowed (-1 for unlimited)
	Low      int // Max low vulnerabilities allowed (-1 for unlimited)
}

// DefaultScanConfig returns sensible defaults for scanning
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		Enabled:        true,
		FailOnCritical: true,
		FailOnHigh:     false,
		FailOnMedium:   false,
		IgnoreUnfixed:  true,
		Timeout:        10 * time.Minute,
		SeverityThresholds: SeverityThresholds{
			Critical: 0,  // No critical vulnerabilities allowed
			High:     -1, // Unlimited high
			Medium:   -1, // Unlimited medium
			Low:      -1, // Unlimited low
		},
	}
}

// Vulnerability represents a single vulnerability finding
type Vulnerability struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgName          string   `json:"PkgName"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion,omitempty"`
	Severity         string   `json:"Severity"`
	Title            string   `json:"Title,omitempty"`
	Description      string   `json:"Description,omitempty"`
	References       []string `json:"References,omitempty"`
	CVSS             *CVSS    `json:"CVSS,omitempty"`
}

// CVSS represents CVSS scoring information
type CVSS struct {
	Score float64 `json:"Score,omitempty"`
}

// ScanResult represents the overall scan result
type ScanResult struct {
	ImageTag        string                     `json:"image_tag"`
	ScanTime        time.Time                  `json:"scan_time"`
	ScanDuration    time.Duration              `json:"scan_duration"`
	VulnCounts      VulnerabilityCounts        `json:"vulnerability_counts"`
	Vulnerabilities []Vulnerability            `json:"vulnerabilities,omitempty"`
	Results         []TrivyResult              `json:"results,omitempty"`
	Pass            bool                       `json:"pass"`
	FailureReason   string                     `json:"failure_reason,omitempty"`
	RawOutput       string                     `json:"raw_output,omitempty"`
	Errors          []string                   `json:"errors,omitempty"`
}

// VulnerabilityCounts tracks vulnerability counts by severity
type VulnerabilityCounts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
	Total    int `json:"total"`
}

// TrivyResult represents a single target result from Trivy
type TrivyResult struct {
	Target          string          `json:"Target"`
	Class           string          `json:"Class,omitempty"`
	Type            string          `json:"Type,omitempty"`
	Vulnerabilities []Vulnerability `json:"Vulnerabilities,omitempty"`
}

// TrivyOutput represents the JSON output from Trivy
type TrivyOutput struct {
	SchemaVersion int           `json:"SchemaVersion"`
	ArtifactName  string        `json:"ArtifactName"`
	ArtifactType  string        `json:"ArtifactType"`
	Results       []TrivyResult `json:"Results,omitempty"`
}

// Scanner interface for vulnerability scanning
type Scanner interface {
	Scan(ctx context.Context, imageTag string) (*ScanResult, error)
	IsAvailable() bool
}

// TrivyScanner implements Scanner using Trivy CLI
type TrivyScanner struct {
	config     ScanConfig
	trivyPath  string
	available  bool
}

// NewTrivyScanner creates a new Trivy scanner
func NewTrivyScanner(config ScanConfig) *TrivyScanner {
	scanner := &TrivyScanner{
		config: config,
	}

	// Find Trivy binary
	trivyPath, err := exec.LookPath("trivy")
	if err != nil {
		log.Warn().Msg("Trivy not found in PATH, vulnerability scanning will be disabled")
		scanner.available = false
	} else {
		scanner.trivyPath = trivyPath
		scanner.available = true
		log.Info().Str("path", trivyPath).Msg("Trivy scanner initialized")
	}

	return scanner
}

// IsAvailable returns whether Trivy is available
func (s *TrivyScanner) IsAvailable() bool {
	return s.available && s.config.Enabled
}

// Scan performs vulnerability scanning on the given image
func (s *TrivyScanner) Scan(ctx context.Context, imageTag string) (*ScanResult, error) {
	startTime := time.Now()

	result := &ScanResult{
		ImageTag: imageTag,
		ScanTime: startTime,
		Pass:     true,
	}

	if !s.IsAvailable() {
		result.Pass = true
		result.FailureReason = "Trivy scanner not available, skipping vulnerability scan"
		log.Warn().Msg(result.FailureReason)
		return result, nil
	}

	log.Info().
		Str("image", imageTag).
		Msg("Starting vulnerability scan with Trivy")

	// Build Trivy command
	args := []string{
		"image",
		"--format", "json",
		"--quiet",
	}

	// Add severity filter
	severities := []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}
	args = append(args, "--severity", strings.Join(severities, ","))

	// Ignore unfixed vulnerabilities if configured
	if s.config.IgnoreUnfixed {
		args = append(args, "--ignore-unfixed")
	}

	args = append(args, imageTag)

	// Create command with context and timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.trivyPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Debug().
		Str("command", s.trivyPath).
		Strs("args", args).
		Msg("Running Trivy command")

	// Run the scan
	err := cmd.Run()
	result.ScanDuration = time.Since(startTime)
	result.RawOutput = stdout.String()

	if ctx.Err() == context.DeadlineExceeded {
		result.Pass = false
		result.FailureReason = "Vulnerability scan timed out"
		result.Errors = append(result.Errors, result.FailureReason)
		return result, fmt.Errorf("scan timed out after %v", s.config.Timeout)
	}

	if err != nil {
		// Trivy exits with non-zero if vulnerabilities found
		// We still want to parse the output
		if stderr.Len() > 0 {
			result.Errors = append(result.Errors, stderr.String())
		}
		log.Debug().Err(err).Str("stderr", stderr.String()).Msg("Trivy command returned error")
	}

	// Parse Trivy output
	var trivyOutput TrivyOutput
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &trivyOutput); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to parse Trivy output: %v", err))
			log.Error().Err(err).Msg("Failed to parse Trivy JSON output")
			// Don't fail the scan for parse errors, just log them
		}
	}

	// Process results
	result.Results = trivyOutput.Results
	result.VulnCounts = s.countVulnerabilities(trivyOutput.Results)
	result.Vulnerabilities = s.extractAllVulnerabilities(trivyOutput.Results)

	// Check against thresholds
	pass, reason := s.checkThresholds(result.VulnCounts)
	result.Pass = pass
	result.FailureReason = reason

	log.Info().
		Str("image", imageTag).
		Int("critical", result.VulnCounts.Critical).
		Int("high", result.VulnCounts.High).
		Int("medium", result.VulnCounts.Medium).
		Int("low", result.VulnCounts.Low).
		Bool("pass", result.Pass).
		Dur("duration", result.ScanDuration).
		Msg("Vulnerability scan completed")

	if !result.Pass {
		return result, fmt.Errorf("vulnerability scan failed: %s", result.FailureReason)
	}

	return result, nil
}

// countVulnerabilities counts vulnerabilities by severity
func (s *TrivyScanner) countVulnerabilities(results []TrivyResult) VulnerabilityCounts {
	counts := VulnerabilityCounts{}

	for _, result := range results {
		for _, vuln := range result.Vulnerabilities {
			switch strings.ToUpper(vuln.Severity) {
			case "CRITICAL":
				counts.Critical++
			case "HIGH":
				counts.High++
			case "MEDIUM":
				counts.Medium++
			case "LOW":
				counts.Low++
			default:
				counts.Unknown++
			}
			counts.Total++
		}
	}

	return counts
}

// extractAllVulnerabilities flattens all vulnerabilities into a single slice
func (s *TrivyScanner) extractAllVulnerabilities(results []TrivyResult) []Vulnerability {
	var vulns []Vulnerability
	for _, result := range results {
		vulns = append(vulns, result.Vulnerabilities...)
	}
	return vulns
}

// checkThresholds checks if the vulnerability counts exceed configured thresholds
func (s *TrivyScanner) checkThresholds(counts VulnerabilityCounts) (bool, string) {
	thresholds := s.config.SeverityThresholds

	// Check critical vulnerabilities
	if s.config.FailOnCritical && counts.Critical > 0 {
		return false, fmt.Sprintf("Found %d critical vulnerabilities (max allowed: 0)", counts.Critical)
	}
	if thresholds.Critical >= 0 && counts.Critical > thresholds.Critical {
		return false, fmt.Sprintf("Found %d critical vulnerabilities (max allowed: %d)", counts.Critical, thresholds.Critical)
	}

	// Check high vulnerabilities
	if s.config.FailOnHigh && counts.High > 0 {
		return false, fmt.Sprintf("Found %d high vulnerabilities (max allowed: 0)", counts.High)
	}
	if thresholds.High >= 0 && counts.High > thresholds.High {
		return false, fmt.Sprintf("Found %d high vulnerabilities (max allowed: %d)", counts.High, thresholds.High)
	}

	// Check medium vulnerabilities
	if s.config.FailOnMedium && counts.Medium > 0 {
		return false, fmt.Sprintf("Found %d medium vulnerabilities (max allowed: 0)", counts.Medium)
	}
	if thresholds.Medium >= 0 && counts.Medium > thresholds.Medium {
		return false, fmt.Sprintf("Found %d medium vulnerabilities (max allowed: %d)", counts.Medium, thresholds.Medium)
	}

	// Check low vulnerabilities
	if thresholds.Low >= 0 && counts.Low > thresholds.Low {
		return false, fmt.Sprintf("Found %d low vulnerabilities (max allowed: %d)", counts.Low, thresholds.Low)
	}

	return true, ""
}

// FormatSummary returns a human-readable summary of scan results
func (r *ScanResult) FormatSummary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Vulnerability Scan Results for %s\n", r.ImageTag))
	sb.WriteString(fmt.Sprintf("Scan Time: %s\n", r.ScanTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Duration: %v\n", r.ScanDuration))
	sb.WriteString("\n")
	sb.WriteString("Vulnerability Summary:\n")
	sb.WriteString(fmt.Sprintf("  Critical: %d\n", r.VulnCounts.Critical))
	sb.WriteString(fmt.Sprintf("  High:     %d\n", r.VulnCounts.High))
	sb.WriteString(fmt.Sprintf("  Medium:   %d\n", r.VulnCounts.Medium))
	sb.WriteString(fmt.Sprintf("  Low:      %d\n", r.VulnCounts.Low))
	sb.WriteString(fmt.Sprintf("  Total:    %d\n", r.VulnCounts.Total))
	sb.WriteString("\n")

	if r.Pass {
		sb.WriteString("Status: PASS\n")
	} else {
		sb.WriteString(fmt.Sprintf("Status: FAIL - %s\n", r.FailureReason))
	}

	return sb.String()
}

// GetTopVulnerabilities returns the most severe vulnerabilities
func (r *ScanResult) GetTopVulnerabilities(limit int) []Vulnerability {
	if limit <= 0 || limit > len(r.Vulnerabilities) {
		limit = len(r.Vulnerabilities)
	}

	// Sort by severity (CRITICAL > HIGH > MEDIUM > LOW)
	severity := map[string]int{
		"CRITICAL": 4,
		"HIGH":     3,
		"MEDIUM":   2,
		"LOW":      1,
		"UNKNOWN":  0,
	}

	vulns := make([]Vulnerability, len(r.Vulnerabilities))
	copy(vulns, r.Vulnerabilities)

	// Simple selection sort (for small n this is fine)
	for i := 0; i < limit && i < len(vulns)-1; i++ {
		maxIdx := i
		for j := i + 1; j < len(vulns); j++ {
			if severity[strings.ToUpper(vulns[j].Severity)] > severity[strings.ToUpper(vulns[maxIdx].Severity)] {
				maxIdx = j
			}
		}
		vulns[i], vulns[maxIdx] = vulns[maxIdx], vulns[i]
	}

	return vulns[:limit]
}
