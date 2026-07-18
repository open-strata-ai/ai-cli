// Package config loads the CLI local configuration fragment
// (infrastructure/config/config.yaml) and resolves effective values from
// environment variables, mirroring SPECS §8.4 / §11.4 / §11.7.
package config

import (
	"os"

	"github.com/open-strata-ai/ai-cli/internal/yaml"
)

// Config is the top-level CLI configuration document.
type Config struct {
	Cli CliSection `yaml:"cli"`
}

// CliSection mirrors the `cli:` mapping in config.yaml.
type CliSection struct {
	DefaultProfile string         `yaml:"defaultProfile"`
	MetaRepo       MetaRepoConfig `yaml:"metaRepo"`
	Platform       PlatformConfig `yaml:"platform"`
	Output         string         `yaml:"output"`
	Timeout        TimeoutConfig  `yaml:"timeout"`
	Debug          DebugConfig    `yaml:"debug"`
}

type MetaRepoConfig struct {
	ProfilesPath string `yaml:"profilesPath"`
}

type PlatformConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type TimeoutConfig struct {
	Up      int `yaml:"up"`
	Ready   int `yaml:"ready"`
	Request int `yaml:"request"`
}

type DebugConfig struct {
	PortRange string `yaml:"portRange"`
}

// Defaults returns the built-in default configuration (SPECS §11.5).
func Defaults() *Config {
	return &Config{Cli: CliSection{
		DefaultProfile: "starter",
		MetaRepo:       MetaRepoConfig{ProfilesPath: "openstrata-meta/profiles"},
		Platform:       PlatformConfig{Endpoint: "http://localhost:8080"},
		Output:         "table",
		Timeout:        TimeoutConfig{Up: 300, Ready: 30, Request: 10},
		Debug:          DebugConfig{PortRange: "8080-8090"},
	}}
}

// Load reads config from path if present, then overlays environment variables.
// A missing file falls back to Defaults (so the CLI runs offline with no config).
func Load(path string) (*Config, error) {
	cfg := Defaults()
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}
	overlayEnv(cfg)
	return cfg, nil
}

func overlayEnv(cfg *Config) {
	if v := os.Getenv("OPENSTRATA_ENDPOINT"); v != "" {
		cfg.Cli.Platform.Endpoint = v
	}
	if v := os.Getenv("OPENSTRATA_PROFILE"); v != "" {
		cfg.Cli.DefaultProfile = v
	}
	if v := os.Getenv("OPENSTRATA_OUTPUT"); v != "" {
		cfg.Cli.Output = v
	}
}

// Effective returns the resolved effective values honoring env > config > flag
// defaults. flag* values are only applied when non-empty.
func (c *Config) Effective(flagProfile, flagEndpoint, flagOutput string) (profile, endpoint, output string) {
	profile = c.Cli.DefaultProfile
	if flagProfile != "" {
		profile = flagProfile
	}
	endpoint = c.Cli.Platform.Endpoint
	if flagEndpoint != "" {
		endpoint = flagEndpoint
	}
	output = c.Cli.Output
	if flagOutput != "" {
		output = flagOutput
	}
	return profile, endpoint, output
}
