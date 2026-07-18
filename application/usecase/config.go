package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
)

// ConfigUseCase reads/writes the local PlatformManifest (SPECS §7.2 / §8.3).
type ConfigUseCase struct {
	svc      *domain.Service
	manifest string
	stateDir string
}

// NewConfigUseCase constructs a ConfigUseCase.
func NewConfigUseCase(svc *domain.Service, manifest, stateDir string) *ConfigUseCase {
	return &ConfigUseCase{svc: svc, manifest: manifest, stateDir: stateDir}
}

// Get reads a manifest field.
func (u *ConfigUseCase) Get(ctx context.Context, key string) (string, error) {
	m, err := u.svc.ReadManifest(u.manifest)
	if err != nil {
		return "", err
	}
	return u.svc.GetManifestValue(m, key)
}

// Set updates a manifest field and re-validates before writing (S3).
func (u *ConfigUseCase) Set(ctx context.Context, key, val string) error {
	m, err := u.svc.ReadManifest(u.manifest)
	if err != nil {
		return err
	}
	if err := u.svc.SetManifestValue(&m, key, val); err != nil {
		return err
	}
	return u.svc.WriteManifest(u.manifest, m)
}

// LoginUseCase stores the API token locally (encrypted) (SKILLS §12.2).
type LoginUseCase struct {
	stateDir string
}

// NewLoginUseCase constructs a LoginUseCase.
func NewLoginUseCase(stateDir string) *LoginUseCase {
	return &LoginUseCase{stateDir: stateDir}
}

// Run encrypts and persists the resolved token, recording the tenant in state.
func (u *LoginUseCase) Run(ctx context.Context, tenant, token string) error {
	if token == "" {
		return domain.ErrAuth("no token provided (set --token, OPENSTRATA_TOKEN, or run `aictl login` interactively)", nil)
	}
	if err := state.SaveToken(u.stateDir, token); err != nil {
		return domain.ErrGeneral("save token", err)
	}
	if tenant != "" {
		st, err := state.LoadState(u.stateDir)
		if err != nil {
			return domain.ErrGeneral("load state", err)
		}
		st.CurrentProfile = tenant
		if err := state.SaveState(u.stateDir, st); err != nil {
			return domain.ErrGeneral("save state", err)
		}
	}
	return nil
}

// LogoutUseCase removes the locally stored token.
type LogoutUseCase struct {
	stateDir string
}

// NewLogoutUseCase constructs a LogoutUseCase.
func NewLogoutUseCase(stateDir string) *LogoutUseCase {
	return &LogoutUseCase{stateDir: stateDir}
}

// Run deletes the encrypted token file if present.
func (u *LogoutUseCase) Run(ctx context.Context) error {
	p := filepath.Join(u.stateDir, "tokens", "jwt.enc")
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return domain.ErrGeneral("remove token", err)
	}
	return nil
}

// DebugUseCase surfaces local runtime info for `aictl debug` (R8).
type DebugUseCase struct {
	stateDir string
}

// NewDebugUseCase constructs a DebugUseCase.
func NewDebugUseCase(stateDir string) *DebugUseCase {
	return &DebugUseCase{stateDir: stateDir}
}

// Run returns local runtime diagnostics (current profile, state, token presence).
func (u *DebugUseCase) Run(ctx context.Context) (map[string]string, error) {
	st, err := state.LoadState(u.stateDir)
	if err != nil {
		return nil, domain.ErrGeneral("load state", err)
	}
	tok, err := state.LoadToken(u.stateDir)
	if err != nil {
		return nil, domain.ErrGeneral("load token", err)
	}
	return map[string]string{
		"state_dir":       u.stateDir,
		"current_profile": st.CurrentProfile,
		"last_checksum":   st.LastChecksum,
		"last_up":         st.LastUpTimestamp,
		"token_present":   fmt.Sprintf("%t", tok != ""),
	}, nil
}
