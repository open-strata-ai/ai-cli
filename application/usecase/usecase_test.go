package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/open-strata-ai/ai-cli/domain"
	"github.com/open-strata-ai/ai-cli/infrastructure/adapter"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
)

func TestInitUseCaseDryRun(t *testing.T) {
	svc := domain.NewService()
	fake := adapter.NewFakePlatformClient()
	uc := NewInitUseCase(svc, fake, t.TempDir())
	m, err := uc.Run(context.Background(), "starter", "qwen-cloud", "acme", true, "openstrata.yaml")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if m.Profile != "starter" || m.Tenant != "acme" {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestInitUseCaseWritesAndRecordsState(t *testing.T) {
	svc := domain.NewService()
	fake := adapter.NewFakePlatformClient()
	dir := t.TempDir()
	manifest := filepath.Join(dir, "openstrata.yaml")
	uc := NewInitUseCase(svc, fake, dir)
	if _, err := uc.Run(context.Background(), "standard", "openai", "acme", false, manifest); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := svc.ReadManifest(manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if got.Model != "openai" {
		t.Fatalf("model = %q", got.Model)
	}
	st, _ := state.LoadState(dir)
	if st.CurrentProfile != "standard" {
		t.Fatalf("state profile = %q", st.CurrentProfile)
	}
}

func TestPlanUseCase(t *testing.T) {
	svc := domain.NewService()
	fake := adapter.NewFakePlatformClient()
	dir := t.TempDir()
	uc := NewPlanUseCase(svc, fake, dir, "")
	cs, err := uc.Run(context.Background(), []string{"gateway", "rag"}, "acme")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if cs == "" {
		t.Fatal("empty checksum")
	}
	st, _ := state.LoadState(dir)
	if st.LastChecksum != cs {
		t.Fatalf("checksum not stored: %q", st.LastChecksum)
	}
}

func TestApplyUseCaseFallsBackToStored(t *testing.T) {
	fake := adapter.NewFakePlatformClient()
	dir := t.TempDir()
	// Seed state with a checksum.
	_ = state.SaveState(dir, &state.State{LastChecksum: "cs_seed"})
	uc := NewApplyUseCase(fake, dir)
	if err := uc.Run(context.Background(), ""); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(fake.Applied) != 1 || fake.Applied[0] != "cs_seed" {
		t.Fatalf("applied = %v", fake.Applied)
	}
}

func TestConfigUseCaseGetSet(t *testing.T) {
	svc := domain.NewService()
	dir := t.TempDir()
	manifest := filepath.Join(dir, "openstrata.yaml")
	m, _ := svc.MergeProfile("starter", "qwen-cloud", "acme", nil)
	if err := svc.WriteManifest(manifest, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	uc := NewConfigUseCase(svc, manifest, dir)
	v, err := uc.Get(context.Background(), "model")
	if err != nil || v != "qwen-cloud" {
		t.Fatalf("get model = %q, err %v", v, err)
	}
	if err := uc.Set(context.Background(), "tenant", "newco"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, _ = uc.Get(context.Background(), "tenant")
	if v != "newco" {
		t.Fatalf("tenant = %q", v)
	}
}

func TestModelUseCase(t *testing.T) {
	fake := adapter.NewFakePlatformClient()
	uc := NewModelUseCase(fake)
	models, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %v", models)
	}
	if err := uc.Enable(context.Background(), "openai"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if len(fake.Enabled) != 1 {
		t.Fatalf("enabled = %v", fake.Enabled)
	}
}

func TestRollbackRequiresComponent(t *testing.T) {
	uc := NewRollbackUseCase(adapter.NewFakePlatformClient())
	if err := uc.Run(context.Background(), ""); err == nil {
		t.Fatal("expected error for missing component")
	}
}
