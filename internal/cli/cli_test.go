package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-strata-ai/ai-cli/infrastructure/adapter"
	"github.com/open-strata-ai/ai-cli/infrastructure/state"
)

func run(t *testing.T, dir, manifest string, args ...string) (int, string) {
	t.Helper()
	client := adapter.NewFakePlatformClient()
	var buf bytes.Buffer
	code := Execute(args, &buf, client)
	return code, buf.String()
}

func TestInitWritesManifestAndState(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, out := run(t, dir, manifest,
		"init", "--profile", "starter", "--model", "qwen-cloud", "--tenant", "acme", "--config", manifest)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "wrote "+manifest) {
		t.Fatalf("unexpected output: %s", out)
	}
	st, err := state.LoadState(dir)
	if err != nil || st.CurrentProfile != "starter" {
		t.Fatalf("state = %+v err %v", st, err)
	}
}

func TestPlanOutputsChecksum(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, out := run(t, dir, manifest, "plan", "--enable", "gateway", "--config", manifest)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "plan checksum:") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestModelList(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, out := run(t, dir, manifest, "model", "list", "--output", "json", "--config", manifest)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "qwen-cloud") {
		t.Fatalf("model list missing qwen-cloud: %s", out)
	}
}

func TestConfigGetSet(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	run(t, dir, manifest, "init", "--profile", "starter", "--model", "qwen-cloud", "--config", manifest)

	code, out := run(t, dir, manifest, "config", "get", "model", "--config", manifest)
	if code != 0 || !strings.Contains(out, "qwen-cloud") {
		t.Fatalf("get model failed: %d %s", code, out)
	}
	code, out = run(t, dir, manifest, "config", "set", "tenant", "newco", "--config", manifest)
	if code != 0 {
		t.Fatalf("set failed: %d %s", code, out)
	}
	_, out = run(t, dir, manifest, "config", "get", "tenant", "--config", manifest)
	if !strings.Contains(out, "newco") {
		t.Fatalf("tenant not updated: %s", out)
	}
}

func TestVersion(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, out := run(t, dir, manifest, "version", "--config", manifest)
	if code != 0 || !strings.Contains(out, "aictl version") {
		t.Fatalf("version failed: %d %s", code, out)
	}
}

func TestDebug(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, out := run(t, dir, manifest, "debug", "--config", manifest)
	if code != 0 || !strings.Contains(out, "current_profile") {
		t.Fatalf("debug failed: %d %s", code, out)
	}
}

func TestHelpNoArgs(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, out := run(t, dir, manifest, "--config", manifest)
	if code != 0 || !strings.Contains(out, "aictl") {
		t.Fatalf("help failed: %d %s", code, out)
	}
}

func TestUnknownCommandExitCode(t *testing.T) {
	dir := t.TempDir()
	stateDirResolver = func() string { return dir }
	manifest := filepath.Join(dir, "openstrata.yaml")
	code, _ := run(t, dir, manifest, "frobnicate", "--config", manifest)
	if code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
	}
}
