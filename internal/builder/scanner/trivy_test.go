package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultScanConfig(t *testing.T) {
	config := DefaultScanConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.FailOnCritical)
	assert.False(t, config.FailOnHigh)
	assert.False(t, config.FailOnMedium)
	assert.True(t, config.IgnoreUnfixed)
	assert.Equal(t, 10*time.Minute, config.Timeout)
	assert.Equal(t, 0, config.SeverityThresholds.Critical)
	assert.Equal(t, -1, config.SeverityThresholds.High)
	assert.Equal(t, -1, config.SeverityThresholds.Medium)
	assert.Equal(t, -1, config.SeverityThresholds.Low)
}

func TestNewTrivyScanner(t *testing.T) {
	config := DefaultScanConfig()
	scanner := NewTrivyScanner(config)

	assert.NotNil(t, scanner)
	// Available status depends on whether Trivy is installed
	// We just verify it doesn't panic
}

func TestTrivyScanner_IsAvailable(t *testing.T) {
	t.Run("disabled config returns false", func(t *testing.T) {
		config := ScanConfig{
			Enabled: false,
		}
		scanner := NewTrivyScanner(config)
		assert.False(t, scanner.IsAvailable())
	})
}

func TestTrivyScanner_CountVulnerabilities(t *testing.T) {
	scanner := &TrivyScanner{}

	results := []TrivyResult{
		{
			Target: "test-image",
			Vulnerabilities: []Vulnerability{
				{VulnerabilityID: "CVE-2021-001", Severity: "CRITICAL"},
				{VulnerabilityID: "CVE-2021-002", Severity: "CRITICAL"},
				{VulnerabilityID: "CVE-2021-003", Severity: "HIGH"},
				{VulnerabilityID: "CVE-2021-004", Severity: "MEDIUM"},
				{VulnerabilityID: "CVE-2021-005", Severity: "LOW"},
				{VulnerabilityID: "CVE-2021-006", Severity: "LOW"},
			},
		},
		{
			Target: "another-layer",
			Vulnerabilities: []Vulnerability{
				{VulnerabilityID: "CVE-2021-007", Severity: "HIGH"},
				{VulnerabilityID: "CVE-2021-008", Severity: "UNKNOWN"},
			},
		},
	}

	counts := scanner.countVulnerabilities(results)

	assert.Equal(t, 2, counts.Critical)
	assert.Equal(t, 2, counts.High)
	assert.Equal(t, 1, counts.Medium)
	assert.Equal(t, 2, counts.Low)
	assert.Equal(t, 1, counts.Unknown)
	assert.Equal(t, 8, counts.Total)
}

func TestTrivyScanner_CheckThresholds(t *testing.T) {
	tests := []struct {
		name       string
		config     ScanConfig
		counts     VulnerabilityCounts
		wantPass   bool
		wantReason string
	}{
		{
			name: "pass when no critical or high vulnerabilities",
			config: ScanConfig{
				Enabled:        true,
				FailOnCritical: true,
				FailOnHigh:     true,
				SeverityThresholds: SeverityThresholds{
					Critical: 0,  // No critical allowed
					High:     0,  // No high allowed
					Medium:   -1, // Unlimited medium
					Low:      -1, // Unlimited low
				},
			},
			counts: VulnerabilityCounts{
				Critical: 0,
				High:     0,
				Medium:   5,
				Low:      10,
			},
			wantPass:   true,
			wantReason: "",
		},
		{
			name: "fail on critical when enabled",
			config: ScanConfig{
				Enabled:        true,
				FailOnCritical: true,
			},
			counts: VulnerabilityCounts{
				Critical: 1,
				High:     5,
			},
			wantPass:   false,
			wantReason: "Found 1 critical vulnerabilities (max allowed: 0)",
		},
		{
			name: "fail on high when enabled",
			config: ScanConfig{
				Enabled:        true,
				FailOnCritical: false,
				FailOnHigh:     true,
			},
			counts: VulnerabilityCounts{
				Critical: 0,
				High:     3,
			},
			wantPass:   false,
			wantReason: "Found 3 high vulnerabilities (max allowed: 0)",
		},
		{
			name: "pass when high disabled",
			config: ScanConfig{
				Enabled:        true,
				FailOnCritical: true,
				FailOnHigh:     false,
				SeverityThresholds: SeverityThresholds{
					Critical: 0,  // No critical allowed
					High:     -1, // Unlimited high
					Medium:   -1, // Unlimited medium
					Low:      -1, // Unlimited low
				},
			},
			counts: VulnerabilityCounts{
				Critical: 0,
				High:     10,
			},
			wantPass:   true,
			wantReason: "",
		},
		{
			name: "respect threshold settings",
			config: ScanConfig{
				Enabled: true,
				SeverityThresholds: SeverityThresholds{
					Critical: 2, // Allow up to 2 critical
					High:     -1,
					Medium:   -1,
					Low:      -1,
				},
			},
			counts: VulnerabilityCounts{
				Critical: 2, // Exactly at threshold
				High:     5,
			},
			wantPass:   true,
			wantReason: "",
		},
		{
			name: "fail when exceeding threshold",
			config: ScanConfig{
				Enabled: true,
				SeverityThresholds: SeverityThresholds{
					Critical: 2,
					High:     -1,
					Medium:   -1,
					Low:      -1,
				},
			},
			counts: VulnerabilityCounts{
				Critical: 3, // Over threshold
				High:     5,
			},
			wantPass:   false,
			wantReason: "Found 3 critical vulnerabilities (max allowed: 2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &TrivyScanner{config: tt.config}
			pass, reason := scanner.checkThresholds(tt.counts)
			assert.Equal(t, tt.wantPass, pass)
			if tt.wantReason != "" {
				assert.Contains(t, reason, tt.wantReason)
			}
		})
	}
}

func TestScanResult_FormatSummary(t *testing.T) {
	result := &ScanResult{
		ImageTag: "test-image:v1",
		ScanTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		VulnCounts: VulnerabilityCounts{
			Critical: 1,
			High:     3,
			Medium:   5,
			Low:      10,
			Total:    19,
		},
		Pass: false,
		FailureReason: "Found critical vulnerabilities",
		ScanDuration: 5 * time.Second,
	}

	summary := result.FormatSummary()

	assert.Contains(t, summary, "test-image:v1")
	assert.Contains(t, summary, "Critical: 1")
	assert.Contains(t, summary, "High:     3")
	assert.Contains(t, summary, "Medium:   5")
	assert.Contains(t, summary, "Low:      10")
	assert.Contains(t, summary, "FAIL")
	assert.Contains(t, summary, "Found critical vulnerabilities")
}

func TestScanResult_GetTopVulnerabilities(t *testing.T) {
	result := &ScanResult{
		Vulnerabilities: []Vulnerability{
			{VulnerabilityID: "CVE-001", Severity: "LOW"},
			{VulnerabilityID: "CVE-002", Severity: "CRITICAL"},
			{VulnerabilityID: "CVE-003", Severity: "MEDIUM"},
			{VulnerabilityID: "CVE-004", Severity: "HIGH"},
			{VulnerabilityID: "CVE-005", Severity: "CRITICAL"},
		},
	}

	top := result.GetTopVulnerabilities(3)

	assert.Len(t, top, 3)
	// First two should be CRITICAL
	assert.Equal(t, "CRITICAL", top[0].Severity)
	assert.Equal(t, "CRITICAL", top[1].Severity)
	// Third should be HIGH
	assert.Equal(t, "HIGH", top[2].Severity)
}

func TestTrivyScanner_ScanUnavailable(t *testing.T) {
	config := ScanConfig{
		Enabled: false, // Disabled
	}
	scanner := NewTrivyScanner(config)

	result, err := scanner.Scan(context.Background(), "test-image:v1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Pass)
	assert.Contains(t, result.FailureReason, "not available")
}

func TestVulnerabilityCounts(t *testing.T) {
	counts := VulnerabilityCounts{
		Critical: 1,
		High:     2,
		Medium:   3,
		Low:      4,
		Unknown:  1,
		Total:    11,
	}

	assert.Equal(t, 1, counts.Critical)
	assert.Equal(t, 2, counts.High)
	assert.Equal(t, 3, counts.Medium)
	assert.Equal(t, 4, counts.Low)
	assert.Equal(t, 1, counts.Unknown)
	assert.Equal(t, 11, counts.Total)
}

func TestExtractAllVulnerabilities(t *testing.T) {
	scanner := &TrivyScanner{}

	results := []TrivyResult{
		{
			Target: "layer1",
			Vulnerabilities: []Vulnerability{
				{VulnerabilityID: "CVE-001"},
				{VulnerabilityID: "CVE-002"},
			},
		},
		{
			Target: "layer2",
			Vulnerabilities: []Vulnerability{
				{VulnerabilityID: "CVE-003"},
			},
		},
		{
			Target:          "layer3",
			Vulnerabilities: nil,
		},
	}

	vulns := scanner.extractAllVulnerabilities(results)

	assert.Len(t, vulns, 3)
	assert.Equal(t, "CVE-001", vulns[0].VulnerabilityID)
	assert.Equal(t, "CVE-002", vulns[1].VulnerabilityID)
	assert.Equal(t, "CVE-003", vulns[2].VulnerabilityID)
}
