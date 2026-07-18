package usecase

import (
	"context"

	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
)

// PlanUseCase previews the assembly plan via the resolver (DESIGN §5.3).
type PlanUseCase struct {
	svc      *domain.Service
	client   domain.PlatformClient
	stateDir string
	manifest string
}

// NewPlanUseCase constructs a PlanUseCase. manifest is the path to
// openstrata.yaml (may be empty when only --enable is used).
func NewPlanUseCase(svc *domain.Service, client domain.PlatformClient, stateDir, manifest string) *PlanUseCase {
	return &PlanUseCase{svc: svc, client: client, stateDir: stateDir, manifest: manifest}
}

// Run merges manifest-enabled with explicit --enable overrides, calls the
// resolver, and records the returned checksum in local state.
func (u *PlanUseCase) Run(ctx context.Context, enable []string, tenant string) (string, error) {
	enabled := map[string]bool{}
	if u.manifest != "" {
		if m, err := u.svc.ReadManifest(u.manifest); err == nil {
			for k, v := range m.Enabled {
				enabled[k] = v
			}
			if tenant == "" {
				tenant = m.Tenant
			}
		}
	}
	for _, e := range enable {
		enabled[e] = true
	}
	list := make([]string, 0, len(enabled))
	for k := range enabled {
		list = append(list, k)
	}
	cs, err := u.client.Plan(ctx, list, tenant)
	if err != nil {
		return "", err
	}
	st, err := state.LoadState(u.stateDir)
	if err != nil {
		return "", domain.ErrGeneral("load state", err)
	}
	st.LastChecksum = cs
	if err := state.SaveState(u.stateDir, st); err != nil {
		return "", domain.ErrGeneral("save state", err)
	}
	return cs, nil
}

// ApplyUseCase applies a previously computed plan (DESIGN §7.1).
type ApplyUseCase struct {
	client   domain.PlatformClient
	stateDir string
}

// NewApplyUseCase constructs an ApplyUseCase.
func NewApplyUseCase(client domain.PlatformClient, stateDir string) *ApplyUseCase {
	return &ApplyUseCase{client: client, stateDir: stateDir}
}

// Run applies the given checksum, falling back to the last stored checksum.
func (u *ApplyUseCase) Run(ctx context.Context, checksum string) error {
	if checksum == "" {
		st, err := state.LoadState(u.stateDir)
		if err != nil {
			return domain.ErrGeneral("load state", err)
		}
		checksum = st.LastChecksum
	}
	if checksum == "" {
		return domain.ErrConfig("no plan checksum provided (use --plan or run `aictl plan`)", nil)
	}
	return u.client.Apply(ctx, checksum)
}
