package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-strata-ai/ai-cli/internal/yaml"
)

// Service holds the pure, local business logic of the CLI: profile merging,
// manifest schema validation, and manifest read/write.
type Service struct{}

// NewService constructs a domain Service.
func NewService() *Service { return &Service{} }

// MergeProfile builds a PlatformManifest from a profile skeleton plus the
// user's model/tenant/enable choices, honoring the priority order in SKILLS §5.5
// (CLI flags > manifest > profile skeleton > global default).
func (s *Service) MergeProfile(profile, model, tenant string, overrides map[string]bool) (Manifest, error) {
	if !isValidProfile(profile) {
		return Manifest{}, ErrConfig(fmt.Sprintf("invalid profile %q (want one of %v)", profile, KnownProfiles), nil)
	}
	skel, ok := DefaultProfiles[profile]
	if !ok {
		return Manifest{}, ErrConfig(fmt.Sprintf("profile %q not found", profile), nil)
	}

	enabled := map[string]bool{}
	for k, v := range skel.Enabled {
		enabled[k] = v
	}
	// Apply explicit --enable overrides (only known capabilities accepted).
	for k, v := range overrides {
		if !KnownCapabilities[k] {
			return Manifest{}, ErrConfig(fmt.Sprintf("unknown capability %q in --enable", k), nil)
		}
		enabled[k] = v
	}

	if model == "" {
		model = skel.ModelDefault
	}
	if model == "" {
		return Manifest{}, ErrConfig("model must be specified (no default available)", nil)
	}
	if tenant == "" {
		tenant = "default"
	}

	m := Manifest{
		Profile: profile,
		Version: "v1.0.0",
		Tenant:  tenant,
		Model:   model,
		Enabled: enabled,
	}
	if err := s.ValidateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// ValidateManifest enforces the PlatformManifest schema (SPECS §8.3).
func (s *Service) ValidateManifest(m Manifest) error {
	if !isValidProfile(m.Profile) {
		return ErrConfig(fmt.Sprintf("invalid profile %q", m.Profile), nil)
	}
	if strings.TrimSpace(m.Model) == "" {
		return ErrConfig("model must not be empty", nil)
	}
	if !isAlnum(m.Tenant) {
		return ErrConfig("tenant must be non-empty and alphanumeric", nil)
	}
	for k := range m.Enabled {
		if !KnownCapabilities[k] {
			return ErrConfig(fmt.Sprintf("enabled key %q is not a known capability", k), nil)
		}
	}
	return nil
}

// WriteManifest serializes and writes a manifest to path with path-traversal
// protection (SKILLS §12.2 S7).
func (s *Service) WriteManifest(path string, m Manifest) error {
	clean, err := safePath(path)
	if err != nil {
		return ErrConfig("unsafe manifest path", err)
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return ErrConfig("marshal manifest", err)
	}
	if err := os.WriteFile(clean, data, 0o644); err != nil {
		return ErrGeneral("write manifest", err)
	}
	return nil
}

// ReadManifest reads and validates a manifest from path.
func (s *Service) ReadManifest(path string) (Manifest, error) {
	clean, err := safePath(path)
	if err != nil {
		return Manifest{}, ErrConfig("unsafe manifest path", err)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return Manifest{}, ErrConfig(fmt.Sprintf("read manifest %q", path), err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, ErrConfig("parse manifest", err)
	}
	if err := s.ValidateManifest(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Supported config keys for `aictl config get/set` (SPECS §7.2).
var ConfigKeys = []string{"profile", "version", "tenant", "model"}

// GetManifestValue returns a manifest field as a string.
func (s *Service) GetManifestValue(m Manifest, key string) (string, error) {
	switch key {
	case "profile":
		return m.Profile, nil
	case "version":
		return m.Version, nil
	case "tenant":
		return m.Tenant, nil
	case "model":
		return m.Model, nil
	case "enabled":
		keys := make([]string, 0, len(m.Enabled))
		for k := range m.Enabled {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return strings.Join(keys, ","), nil
	default:
		return "", ErrConfig(fmt.Sprintf("unknown config key %q (want one of %v)", key, ConfigKeys), nil)
	}
}

// SetManifestValue sets a scalar manifest field, then re-validates (SKILLS §12.2 S3).
func (s *Service) SetManifestValue(m *Manifest, key, val string) error {
	switch key {
	case "profile":
		if !isValidProfile(val) {
			return ErrConfig(fmt.Sprintf("invalid profile %q", val), nil)
		}
		m.Profile = val
	case "version":
		m.Version = val
	case "tenant":
		if !isAlnum(val) {
			return ErrConfig("tenant must be alphanumeric", nil)
		}
		m.Tenant = val
	case "model":
		if strings.TrimSpace(val) == "" {
			return ErrConfig("model must not be empty", nil)
		}
		m.Model = val
	default:
		return ErrConfig(fmt.Sprintf("unknown config key %q", key), nil)
	}
	return s.ValidateManifest(*m)
}

func isValidProfile(p string) bool {
	for _, k := range KnownProfiles {
		if k == p {
			return true
		}
	}
	return false
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// safePath rejects absolute traversal outside the current directory for
// relative inputs, preventing "../../" escape (SKILLS §12.2 S7).
func safePath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path %q escapes working directory", p)
	}
	return clean, nil
}
