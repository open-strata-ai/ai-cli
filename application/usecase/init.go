// Package usecase orchestrates domain logic, the platform client, and local
// state into the command behaviors defined in DESIGN §2 / ARCH §3.3.
package usecase

import (
	"context"
	"time"

	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
)

// InitUseCase generates openstrata.yaml from a profile skeleton (DESIGN §5.1).
type InitUseCase struct {
	svc      *domain.Service
	client   domain.PlatformClient
	stateDir string
}

// NewInitUseCase constructs an InitUseCase.
func NewInitUseCase(svc *domain.Service, client domain.PlatformClient, stateDir string) *InitUseCase {
	return &InitUseCase{svc: svc, client: client, stateDir: stateDir}
}

// Run merges the profile into a manifest and writes it (unless dryRun), then
// records the current profile in local state.
func (u *InitUseCase) Run(ctx context.Context, profile, model, tenant string, dryRun bool, manifestPath string) (domain.Manifest, error) {
	m, err := u.svc.MergeProfile(profile, model, tenant, nil)
	if err != nil {
		return domain.Manifest{}, err
	}
	if dryRun {
		return m, nil
	}
	if err := u.svc.WriteManifest(manifestPath, m); err != nil {
		return domain.Manifest{}, err
	}
	st, err := state.LoadState(u.stateDir)
	if err != nil {
		return domain.Manifest{}, domain.ErrGeneral("load state", err)
	}
	st.CurrentProfile = profile
	if err := state.SaveState(u.stateDir, st); err != nil {
		return domain.Manifest{}, domain.ErrGeneral("save state", err)
	}
	return m, nil
}

// UpUseCase pulls up the platform core components (DESIGN §5.2).
type UpUseCase struct {
	client   domain.PlatformClient
	stateDir string
}

// NewUpUseCase constructs an UpUseCase.
func NewUpUseCase(client domain.PlatformClient, stateDir string) *UpUseCase {
	return &UpUseCase{client: client, stateDir: stateDir}
}

// Run triggers the platform pull-up and records the timestamp in local state.
func (u *UpUseCase) Run(ctx context.Context, profile string) error {
	if err := u.client.Up(ctx, profile); err != nil {
		return err
	}
	st, err := state.LoadState(u.stateDir)
	if err != nil {
		return domain.ErrGeneral("load state", err)
	}
	st.CurrentProfile = profile
	st.LastUpTimestamp = time.Now().UTC().Format(time.RFC3339)
	return state.SaveState(u.stateDir, st)
}

// RollbackUseCase rolls a component back (DESIGN §7.1).
type RollbackUseCase struct {
	client domain.PlatformClient
}

// NewRollbackUseCase constructs a RollbackUseCase.
func NewRollbackUseCase(client domain.PlatformClient) *RollbackUseCase {
	return &RollbackUseCase{client: client}
}

// Run rolls back the named component.
func (u *RollbackUseCase) Run(ctx context.Context, component string) error {
	if component == "" {
		return domain.ErrConfig("rollback requires --component", nil)
	}
	return u.client.Rollback(ctx, component)
}
