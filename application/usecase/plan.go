package usecase

import (
	"context"
	"encoding/json"

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
	cs, planRaw, err := u.client.Plan(ctx, list, tenant)
	if err != nil {
		return "", err
	}
	st, err := state.LoadState(u.stateDir)
	if err != nil {
		return "", domain.ErrGeneral("load state", err)
	}
	st.LastChecksum = cs
	st.LastPlan = planRaw
	st.LastTenant = tenant
	st.LastProfile = "starter"
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

// Run applies a plan. If checksum is given it is fetched from the resolver;
// otherwise the last plan stored by `aictl plan` is used.
func (u *ApplyUseCase) Run(ctx context.Context, checksum string) error {
	st, err := state.LoadState(u.stateDir)
	if err != nil {
		return domain.ErrGeneral("load state", err)
	}
	var planRaw json.RawMessage
	if checksum != "" {
		planRaw, err = u.client.GetPlan(ctx, checksum)
		if err != nil {
			return err
		}
	} else if len(st.LastPlan) > 0 {
		planRaw = st.LastPlan
	} else if st.LastChecksum != "" {
		planRaw, err = u.client.GetPlan(ctx, st.LastChecksum)
		if err != nil {
			return err
		}
	} else {
		return domain.ErrConfig("no plan to apply (run `aictl plan` first)", nil)
	}
	profile := st.LastProfile
	if profile == "" {
		profile = "starter"
	}
	tenantID := st.LastTenant
	if tenantID == "" {
		tenantID = "local"
	}
	return u.client.Apply(ctx, planRaw, profile, tenantID)
}
