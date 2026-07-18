package adapter

import (
	"context"
	"strings"
	"testing"

	"github.com/open-strata-ai/ai-cli/domain"
)

func TestFakePlatformClientPlanApply(t *testing.T) {
	f := NewFakePlatformClient()
	cs, _, err := f.Plan(context.Background(), []string{"gateway", "llm"}, "acme")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if cs == "" {
		t.Fatal("empty checksum")
	}
	// Deterministic for same input.
	cs2, _, _ := f.Plan(context.Background(), []string{"gateway", "llm"}, "acme")
	if cs != cs2 {
		t.Fatalf("checksum not deterministic: %q vs %q", cs, cs2)
	}
	if err := f.Apply(context.Background(), nil, "starter", "acme"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(f.Applied) != 1 {
		t.Fatalf("applied = %v", f.Applied)
	}
}

func TestFakePlatformClientModels(t *testing.T) {
	f := NewFakePlatformClient()
	models, err := f.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("models = %v", models)
	}
	if err := f.EnableModel(context.Background(), "openai"); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if len(f.Enabled) != 1 || f.Enabled[0] != "openai" {
		t.Fatalf("enabled = %v", f.Enabled)
	}
}

func TestFakePlatformClientUpRollbackEval(t *testing.T) {
	f := NewFakePlatformClient()
	if err := f.Up(context.Background(), "starter"); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := f.Rollback(context.Background(), "gateway"); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	id, err := f.RunEval(context.Background(), "task.yaml")
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if id == "" {
		t.Fatal("empty task id")
	}
	res, err := f.EvalStatus(context.Background(), id)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if res.Status != "done" {
		t.Fatalf("status = %q", res.Status)
	}
}

func TestFakePlatformClientLogs(t *testing.T) {
	f := NewFakePlatformClient()
	r, err := f.AppLogs(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	defer r.Close()
	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	if !strings.Contains(string(buf[:n]), "myapp") {
		t.Fatalf("logs = %q", string(buf[:n]))
	}
}

func TestFakeImplementsPort(t *testing.T) {
	var _ domain.PlatformClient = NewFakePlatformClient()
	var _ domain.PlatformClient = NewPlatformClient("http://localhost:8080")
}
