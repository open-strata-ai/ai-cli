package state

import (
	"path/filepath"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	plain := "eyJ.very.secret.token"
	sealed, err := SealToken(plain)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if sealed == plain {
		t.Fatal("token was not encrypted")
	}
	open, err := OpenToken(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if open != plain {
		t.Fatalf("round-trip mismatch: got %q", open)
	}
}

func TestSaveLoadToken(t *testing.T) {
	dir := t.TempDir()
	if err := SaveToken(dir, "tok123"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadToken(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "tok123" {
		t.Fatalf("token = %q", got)
	}
	// Absent token returns empty string.
	if v, _ := LoadToken(filepath.Join(dir, "nope")); v != "" {
		t.Fatalf("expected empty for missing token, got %q", v)
	}
}

func TestResolveTokenPriority(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENSTRATA_TOKEN", "envtok")
	if got := ResolveToken(dir, ""); got != "envtok" {
		t.Fatalf("env should win, got %q", got)
	}
	if got := ResolveToken(dir, "flagtok"); got != "flagtok" {
		t.Fatalf("flag should win over env, got %q", got)
	}
	// No env, no flag, stored token.
	t.Setenv("OPENSTRATA_TOKEN", "")
	_ = SaveToken(dir, "stored")
	if got := ResolveToken(dir, ""); got != "stored" {
		t.Fatalf("stored should be used, got %q", got)
	}
}

func TestStateSaveLoad(t *testing.T) {
	dir := t.TempDir()
	st := &State{CurrentProfile: "advanced", LastChecksum: "cs1"}
	if err := SaveState(dir, st); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadState(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.CurrentProfile != "advanced" || got.LastChecksum != "cs1" {
		t.Fatalf("state = %+v", got)
	}
	// Missing state returns empty.
	if s, _ := LoadState(filepath.Join(dir, "missing")); s.CurrentProfile != "" {
		t.Fatalf("expected empty state, got %+v", s)
	}
}
