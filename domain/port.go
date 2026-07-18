package domain

import (
	"context"
	"encoding/json"
)

// PlatformClient is the core port for interacting with the OpenStrata platform.
// The anti-corrosion layer implements it in infrastructure/adapter; use cases
// and commands depend only on this interface.
type PlatformClient interface {
	// Bootstrap / pull-up
	Init(ctx context.Context, profile, model string) error
	Up(ctx context.Context, profile string) error

	// Assembly orchestration (pass-through to resolver / provisioner).
	// Plan returns the resolver checksum AND the resolved plan JSON so the
	// CLI can forward the real plan object to the provisioner (which expects
	// an AssemblyPlan, not a checksum string).
	Plan(ctx context.Context, enable []string, tenant string) (checksum string, plan json.RawMessage, err error)
	// GetPlan fetches a previously resolved plan by checksum from the resolver.
	GetPlan(ctx context.Context, checksum string) (json.RawMessage, error)
	// Apply submits a resolved plan object (AssemblyPlan) to the provisioner.
	Apply(ctx context.Context, plan json.RawMessage, profile, tenantID string) error
	Rollback(ctx context.Context, component string) error

	// Model management (via gateway)
	ListModels(ctx context.Context) ([]ModelView, error)
	EnableModel(ctx context.Context, modelID string) error
	DisableModel(ctx context.Context, modelID string) error

	// Application deployment (via platform-api)
	DeployApp(ctx context.Context, specPath string) error
	AppLogs(ctx context.Context, appName string) (ReadCloser, error)

	// Evaluation (via eval-service)
	RunEval(ctx context.Context, taskPath string) (string, error) // returns taskID
	EvalStatus(ctx context.Context, taskID string) (EvalTaskResult, error)
	EvalResults(ctx context.Context, taskID string) (EvalTaskResult, error)

	// Configuration management (read/write PlatformManifest remotely or locally)
	GetConfig(ctx context.Context, key string) (string, error)
	SetConfig(ctx context.Context, key, val string) error

	// Debug
	PortForward(ctx context.Context, appName string, port int) error
}
