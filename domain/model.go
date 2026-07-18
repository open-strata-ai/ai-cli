// Package domain holds the pure business types and rules of the aictl CLI.
// It has zero external dependencies so it is fully unit-testable offline.
package domain

import "io"

// CLIContext aggregates the context of the current command invocation.
type CLIContext struct {
	Profile    string // starter|standard|advanced|full (resolved)
	Manifest   ManifestRef
	Endpoint   string // platform control-plane address
	Output     string // table|json|yaml
	Verbose    bool
	NoColor    bool
	ConfigPath string // path to openstrata.yaml
}

// ManifestRef is a reference to the local PlatformManifest file.
type ManifestRef struct {
	Path   string
	Parsed Manifest
}

// Manifest is the resolved view of openstrata.yaml (PlatformManifest).
type Manifest struct {
	Profile string          `yaml:"profile"`
	Version string          `yaml:"version"`
	Tenant  string          `yaml:"tenant"`
	Model   string          `yaml:"model"`
	Enabled map[string]bool `yaml:"enabled"`
}

// ModelView describes a model supplier as seen by the gateway.
type ModelView struct {
	ModelID string `json:"model_id" yaml:"model_id"`
	Source  string `json:"source" yaml:"source"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Health  string `json:"health" yaml:"health"`
	Latency string `json:"latency_ms" yaml:"latency_ms"`
}

// EvalTaskResult is a summary of an evaluation task.
type EvalTaskResult struct {
	TaskID   string  `json:"task_id" yaml:"task_id"`
	Status   string  `json:"status" yaml:"status"`
	Score    float64 `json:"score" yaml:"score"`
	Duration string  `json:"duration" yaml:"duration"`
}

// Profile is the skeleton shipped by openstrata-meta/profiles/<p>.yaml.
// Embedded here as the offline anti-corrosion stand-in for the meta repository.
type Profile struct {
	External         []string        `yaml:"external"`
	OptionalDisabled []string        `yaml:"optional_disabled"`
	Enabled          map[string]bool `yaml:"enabled"`
	ModelDefault     string          `yaml:"model_default"`
}

// DefaultProfiles is the offline stand-in for openstrata-meta/profiles/*.yaml.
// It mirrors the dependency-resolver profile component sets (milvus omitted to
// stay conflict-free, per ai-dependency-resolver DESIGN §5.2).
var DefaultProfiles = map[string]Profile{
	"starter": {
		External:         []string{"gateway", "llm", "memory"},
		OptionalDisabled: []string{"rag", "qdrant", "sandbox", "minio"},
		Enabled:          map[string]bool{"gateway": true, "llm": true, "memory": true},
		ModelDefault:     "qwen-cloud",
	},
	"standard": {
		External:         []string{"gateway", "llm", "memory", "rag", "qdrant"},
		OptionalDisabled: []string{"sandbox", "minio"},
		Enabled:          map[string]bool{"gateway": true, "llm": true, "memory": true, "rag": true, "qdrant": true},
		ModelDefault:     "qwen-cloud",
	},
	"advanced": {
		External:         []string{"gateway", "llm", "memory", "rag", "qdrant", "sandbox", "minio"},
		OptionalDisabled: []string{},
		Enabled:          map[string]bool{"gateway": true, "llm": true, "memory": true, "rag": true, "qdrant": true, "sandbox": true, "minio": true},
		ModelDefault:     "qwen-cloud",
	},
	"full": {
		External:         []string{"gateway", "llm", "memory", "rag", "qdrant", "sandbox", "minio"},
		OptionalDisabled: []string{},
		Enabled:          map[string]bool{"gateway": true, "llm": true, "memory": true, "rag": true, "qdrant": true, "sandbox": true, "minio": true},
		ModelDefault:     "qwen-cloud",
	},
}

// KnownProfiles enumerates the four supported profiles.
var KnownProfiles = []string{"starter", "standard", "advanced", "full"}

// KnownCapabilities is the union of every capability key referenced by any
// profile. Used by schema validation so profile-derived manifests always pass.
var KnownCapabilities = func() map[string]bool {
	set := map[string]bool{}
	for _, p := range DefaultProfiles {
		for k := range p.Enabled {
			set[k] = true
		}
		for _, k := range p.External {
			set[k] = true
		}
		for _, k := range p.OptionalDisabled {
			set[k] = true
		}
	}
	for _, k := range []string{"ui", "agent-engine", "milvus"} {
		set[k] = true
	}
	return set
}()

// ReadCloser is re-exported for adapter signatures that stream logs.
type ReadCloser = io.ReadCloser
