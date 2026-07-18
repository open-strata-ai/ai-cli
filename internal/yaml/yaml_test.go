package yaml

import (
	"strings"
	"testing"
)

type inner struct {
	B bool `yaml:"b"`
	N int  `yaml:"n"`
}

type sample struct {
	Name    string            `yaml:"name"`
	Count   int               `yaml:"count"`
	Enabled bool              `yaml:"enabled"`
	Tags    []string          `yaml:"tags"`
	Meta    map[string]string `yaml:"meta"`
	Inner   inner             `yaml:"inner"`
	Empty   string            `yaml:"empty,omitempty"`
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	in := sample{
		Name:    "openstrata",
		Count:   3,
		Enabled: true,
		Tags:    []string{"a", "b:c", "d e"},
		Meta:    map[string]string{"k1": "v1", "k2": "v2"},
		Inner:   inner{B: false, N: 7},
	}
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out sample
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v\n---\n%s", err, data)
	}
	if out.Name != in.Name || out.Count != in.Count || out.Enabled != in.Enabled ||
		len(out.Tags) != len(in.Tags) || out.Meta["k1"] != "v1" || out.Inner != in.Inner {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func TestMarshalOmitempty(t *testing.T) {
	data, _ := Marshal(sample{Name: "x"}) // Empty omitted
	s := string(data)
	if strings.Contains(s, "empty:") {
		t.Fatalf("omitempty failed, got:\n%s", s)
	}
}

func TestUnmarshalNestedConfig(t *testing.T) {
	src := `
cli:
  defaultProfile: starter
  platform:
    endpoint: http://localhost:8080
  output: table
  timeout:
    up: 300
    ready: 30
`
	var root struct {
		Cli struct {
			DefaultProfile string `yaml:"defaultProfile"`
			Platform       struct {
				Endpoint string `yaml:"endpoint"`
			} `yaml:"platform"`
			Output  string `yaml:"output"`
			Timeout struct {
				Up    int `yaml:"up"`
				Ready int `yaml:"ready"`
			} `yaml:"timeout"`
		} `yaml:"cli"`
	}
	if err := Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if root.Cli.DefaultProfile != "starter" {
		t.Fatalf("defaultProfile = %q", root.Cli.DefaultProfile)
	}
	if root.Cli.Platform.Endpoint != "http://localhost:8080" {
		t.Fatalf("endpoint = %q", root.Cli.Platform.Endpoint)
	}
	if root.Cli.Timeout.Up != 300 || root.Cli.Timeout.Ready != 30 {
		t.Fatalf("timeout = %+v", root.Cli.Timeout)
	}
}

func TestUnmarshalManifest(t *testing.T) {
	src := `
profile: starter
version: v1.0.0
tenant: default
model: qwen-cloud
enabled:
  gateway: true
  llm: true
  memory: false
`
	var m struct {
		Profile string          `yaml:"profile"`
		Version string          `yaml:"version"`
		Tenant  string          `yaml:"tenant"`
		Model   string          `yaml:"model"`
		Enabled map[string]bool `yaml:"enabled"`
	}
	if err := Unmarshal([]byte(src), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Profile != "starter" || m.Model != "qwen-cloud" {
		t.Fatalf("manifest = %+v", m)
	}
	if !m.Enabled["gateway"] || m.Enabled["memory"] {
		t.Fatalf("enabled = %+v", m.Enabled)
	}
}

func TestUnmarshalScalarSequence(t *testing.T) {
	src := "items:\n  - one\n  - two\n  - three\n"
	var root struct {
		Items []string `yaml:"items"`
	}
	if err := Unmarshal([]byte(src), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(root.Items) != 3 || root.Items[2] != "three" {
		t.Fatalf("items = %+v", root.Items)
	}
}
