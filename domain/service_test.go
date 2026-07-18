package domain

import (
	"path/filepath"
	"testing"
)

func TestMergeProfileDefaults(t *testing.T) {
	svc := NewService()
	m, err := svc.MergeProfile("starter", "qwen-cloud", "acme", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if m.Profile != "starter" || m.Model != "qwen-cloud" || m.Tenant != "acme" {
		t.Fatalf("manifest = %+v", m)
	}
	for _, want := range []string{"gateway", "llm", "memory"} {
		if !m.Enabled[want] {
			t.Fatalf("starter missing %q, got %v", want, m.Enabled)
		}
	}
}

func TestMergeProfileModelDefault(t *testing.T) {
	svc := NewService()
	m, err := svc.MergeProfile("standard", "", "", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if m.Model != "qwen-cloud" {
		t.Fatalf("expected default model, got %q", m.Model)
	}
	if !m.Enabled["rag"] || !m.Enabled["qdrant"] {
		t.Fatalf("standard should enable rag+qdrant, got %v", m.Enabled)
	}
}

func TestMergeProfileInvalid(t *testing.T) {
	svc := NewService()
	if _, err := svc.MergeProfile("bogus", "", "", nil); err == nil {
		t.Fatal("expected error for invalid profile")
	}
}

func TestMergeProfileOverrideUnknownCapability(t *testing.T) {
	svc := NewService()
	if _, err := svc.MergeProfile("starter", "", "", map[string]bool{"not-a-cap": true}); err == nil {
		t.Fatal("expected error for unknown capability override")
	}
}

func TestValidateManifest(t *testing.T) {
	svc := NewService()
	if err := svc.ValidateManifest(Manifest{Profile: "starter", Model: "x", Tenant: "t01", Enabled: map[string]bool{"gateway": true}}); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if err := svc.ValidateManifest(Manifest{Profile: "starter", Model: "", Tenant: "t", Enabled: map[string]bool{}}); err == nil {
		t.Fatal("expected error for empty model")
	}
	if err := svc.ValidateManifest(Manifest{Profile: "starter", Model: "x", Tenant: "bad tenant", Enabled: map[string]bool{}}); err == nil {
		t.Fatal("expected error for non-alnum tenant")
	}
	if err := svc.ValidateManifest(Manifest{Profile: "starter", Model: "x", Tenant: "t", Enabled: map[string]bool{"bogus": true}}); err == nil {
		t.Fatal("expected error for unknown enabled key")
	}
}

func TestWriteReadManifest(t *testing.T) {
	svc := NewService()
	path := filepath.Join(t.TempDir(), "openstrata.yaml")
	m, _ := svc.MergeProfile("advanced", "openai", "acme", nil)
	if err := svc.WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := svc.ReadManifest(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Tenant != "acme" || got.Model != "openai" {
		t.Fatalf("round-trip = %+v", got)
	}
}

func TestManifestConfigGetSet(t *testing.T) {
	svc := NewService()
	m, _ := svc.MergeProfile("starter", "qwen-cloud", "acme", nil)
	if v, _ := svc.GetManifestValue(m, "model"); v != "qwen-cloud" {
		t.Fatalf("get model = %q", v)
	}
	if err := svc.SetManifestValue(&m, "tenant", "newco"); err != nil {
		t.Fatalf("set tenant: %v", err)
	}
	if m.Tenant != "newco" {
		t.Fatalf("tenant = %q", m.Tenant)
	}
	if err := svc.SetManifestValue(&m, "bogus", "x"); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSafePathTraversal(t *testing.T) {
	if _, err := safePath("../escape.yaml"); err == nil {
		t.Fatal("expected traversal rejection")
	}
	if _, err := safePath("sub/openstrata.yaml"); err != nil {
		t.Fatalf("valid relative path rejected: %v", err)
	}
}

func TestExitCodeOf(t *testing.T) {
	if ExitCodeOf(nil) != 0 {
		t.Fatal("nil error should be 0")
	}
	if ExitCodeOf(ErrConflict("x", nil)) != 4 {
		t.Fatal("conflict should map to 4")
	}
	if ExitCodeOf(ErrAuth("x", nil)) != 5 {
		t.Fatal("auth should map to 5")
	}
	if ExitCodeOf(ErrGeneral("x", nil)) != 1 {
		t.Fatal("general should map to 1")
	}
}
