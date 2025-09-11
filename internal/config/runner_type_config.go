package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gateixeira/live-actions/models"
)

// RunnerTypeConfig holds the configuration for runner type detection
type RunnerTypeConfig struct {
	SelfHostedLabels   []string          `json:"self_hosted_labels"`
	GitHubHostedLabels []string          `json:"github_hosted_labels"`
	DefaultRunnerType  models.RunnerType `json:"default_runner_type"`
}

// LoadRunnerTypeConfig loads runner type configuration from file or returns default
func LoadRunnerTypeConfig(configPath string) (*RunnerTypeConfig, error) {
	// Read and parse the config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runner type config file: %w", err)
	}

	var config RunnerTypeConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse runner type config JSON: %w", err)
	}

	return &config, nil
}

// InferRunnerType determines the runner type based on the provided labels
func (r *RunnerTypeConfig) InferRunnerType(labels []string) models.RunnerType {
	for _, label := range labels {
		for _, selfHostedLabel := range r.SelfHostedLabels {
			if label == selfHostedLabel {
				fmt.Println("Matched self-hosted label:", label)
				return models.RunnerTypeSelfHosted
			}
		}
	}

	for _, label := range labels {
		for _, githubLabel := range r.GitHubHostedLabels {
			if label == githubLabel {
				return models.RunnerTypeGitHubHosted
			}
		}
	}

	return r.DefaultRunnerType
}
