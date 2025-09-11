package config

import (
	"testing"

	"github.com/gateixeira/live-actions/models"
)

func TestRunnerTypeConfig_InferRunnerType(t *testing.T) {
	// Create a test configuration
	config := &RunnerTypeConfig{
		SelfHostedLabels: []string{
			"self-hosted",
			"self-hosted-large",
			"custom-runner",
		},
		GitHubHostedLabels: []string{
			"ubuntu-latest",
			"ubuntu-22.04",
			"windows-latest",
			"macos-latest",
		},
		DefaultRunnerType: models.RunnerTypeUnknown,
	}

	tests := []struct {
		name     string
		labels   []string
		expected models.RunnerType
	}{
		{
			name:     "self-hosted runner with single matching label",
			labels:   []string{"self-hosted"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "self-hosted runner with multiple labels including matching one",
			labels:   []string{"linux", "self-hosted", "x64"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "self-hosted runner with custom label",
			labels:   []string{"custom-runner"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "self-hosted runner with large label",
			labels:   []string{"self-hosted-large", "linux"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "github-hosted runner with ubuntu-latest",
			labels:   []string{"ubuntu-latest"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "github-hosted runner with specific ubuntu version",
			labels:   []string{"ubuntu-22.04"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "github-hosted runner with windows",
			labels:   []string{"windows-latest"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "github-hosted runner with macos",
			labels:   []string{"macos-latest"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "github-hosted runner with multiple labels",
			labels:   []string{"x64", "ubuntu-latest", "linux"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "no matching labels returns default",
			labels:   []string{"unknown-label", "custom-label"},
			expected: models.RunnerTypeUnknown,
		},
		{
			name:     "empty labels returns default",
			labels:   []string{},
			expected: models.RunnerTypeUnknown,
		},
		{
			name:     "nil labels returns default",
			labels:   nil,
			expected: models.RunnerTypeUnknown,
		},
		{
			name:     "self-hosted takes precedence over github-hosted",
			labels:   []string{"ubuntu-latest", "self-hosted"},
			expected: models.RunnerTypeSelfHosted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.InferRunnerType(tt.labels)
			if result != tt.expected {
				t.Errorf("InferRunnerType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRunnerTypeConfig_InferRunnerType_WithDifferentDefaults(t *testing.T) {
	tests := []struct {
		name              string
		defaultRunnerType models.RunnerType
		labels            []string
		expected          models.RunnerType
	}{
		{
			name:              "default to github-hosted when no match",
			defaultRunnerType: models.RunnerTypeGitHubHosted,
			labels:            []string{"unknown-label"},
			expected:          models.RunnerTypeGitHubHosted,
		},
		{
			name:              "default to self-hosted when no match",
			defaultRunnerType: models.RunnerTypeSelfHosted,
			labels:            []string{"unknown-label"},
			expected:          models.RunnerTypeSelfHosted,
		},
		{
			name:              "default to unknown when no match",
			defaultRunnerType: models.RunnerTypeUnknown,
			labels:            []string{"unknown-label"},
			expected:          models.RunnerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RunnerTypeConfig{
				SelfHostedLabels:   []string{"self-hosted"},
				GitHubHostedLabels: []string{"ubuntu-latest"},
				DefaultRunnerType:  tt.defaultRunnerType,
			}

			result := config.InferRunnerType(tt.labels)
			if result != tt.expected {
				t.Errorf("InferRunnerType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRunnerTypeConfig_InferRunnerType_EmptyConfiguration(t *testing.T) {
	config := &RunnerTypeConfig{
		SelfHostedLabels:   []string{},
		GitHubHostedLabels: []string{},
		DefaultRunnerType:  models.RunnerTypeUnknown,
	}

	tests := []struct {
		name     string
		labels   []string
		expected models.RunnerType
	}{
		{
			name:     "any labels with empty config returns default",
			labels:   []string{"ubuntu-latest", "self-hosted"},
			expected: models.RunnerTypeUnknown,
		},
		{
			name:     "empty labels with empty config returns default",
			labels:   []string{},
			expected: models.RunnerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.InferRunnerType(tt.labels)
			if result != tt.expected {
				t.Errorf("InferRunnerType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRunnerTypeConfig_InferRunnerType_CaseSensitivity(t *testing.T) {
	config := &RunnerTypeConfig{
		SelfHostedLabels:   []string{"self-hosted"},
		GitHubHostedLabels: []string{"ubuntu-latest"},
		DefaultRunnerType:  models.RunnerTypeUnknown,
	}

	tests := []struct {
		name     string
		labels   []string
		expected models.RunnerType
	}{
		{
			name:     "exact case match for self-hosted",
			labels:   []string{"self-hosted"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "different case for self-hosted returns default",
			labels:   []string{"Self-Hosted", "SELF-HOSTED"},
			expected: models.RunnerTypeUnknown,
		},
		{
			name:     "exact case match for github-hosted",
			labels:   []string{"ubuntu-latest"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "different case for github-hosted returns default",
			labels:   []string{"Ubuntu-Latest", "UBUNTU-LATEST"},
			expected: models.RunnerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.InferRunnerType(tt.labels)
			if result != tt.expected {
				t.Errorf("InferRunnerType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRunnerTypeConfig_InferRunnerType_MultipleMatches(t *testing.T) {
	config := &RunnerTypeConfig{
		SelfHostedLabels: []string{
			"self-hosted",
			"self-hosted-large",
		},
		GitHubHostedLabels: []string{
			"ubuntu-latest",
			"ubuntu-22.04",
		},
		DefaultRunnerType: models.RunnerTypeUnknown,
	}

	tests := []struct {
		name     string
		labels   []string
		expected models.RunnerType
	}{
		{
			name:     "multiple self-hosted labels returns self-hosted",
			labels:   []string{"self-hosted", "self-hosted-large"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "multiple github-hosted labels returns github-hosted",
			labels:   []string{"ubuntu-latest", "ubuntu-22.04"},
			expected: models.RunnerTypeGitHubHosted,
		},
		{
			name:     "mixed labels with self-hosted first returns self-hosted",
			labels:   []string{"self-hosted", "ubuntu-latest"},
			expected: models.RunnerTypeSelfHosted,
		},
		{
			name:     "mixed labels with github-hosted first but self-hosted present returns self-hosted",
			labels:   []string{"ubuntu-latest", "self-hosted"},
			expected: models.RunnerTypeSelfHosted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.InferRunnerType(tt.labels)
			if result != tt.expected {
				t.Errorf("InferRunnerType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
