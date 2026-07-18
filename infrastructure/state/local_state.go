// Package state manages the CLI local state directory (~/.openstrata/):
// runtime state (state.json) and encrypted API tokens (SKILLS §12.2 S1/S2).
package state

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultDir returns the local state directory (~/.openstrata).
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".openstrata")
}

// State is the runtime status persisted under ~/.openstrata/state.json.
type State struct {
	CurrentProfile  string          `json:"current_profile"`
	LastChecksum    string          `json:"last_checksum"`
	LastPlan        json.RawMessage `json:"last_plan"`
	LastTenant      string          `json:"last_tenant"`
	LastProfile     string          `json:"last_profile"`
	LastUpTimestamp string          `json:"last_up_timestamp"`
}

// LoadState reads state from dir; returns an empty state if absent.
func LoadState(dir string) (*State, error) {
	p := filepath.Join(dir, "state.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{}, nil
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveState writes state to dir (creating it with 0700 perms).
func SaveState(dir string, s *State) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "state.json"), data, 0o600)
}

// ---------------------------------------------------------------------------
// Token encryption (AES-GCM, machine-bound key)
// ---------------------------------------------------------------------------

func deriveKey() []byte {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "openstrata"
	}
	sum := sha256.Sum256([]byte("openstrata-cli:" + host))
	return sum[:]
}

// SealToken encrypts a plaintext token with AES-GCM and returns base64 text.
func SealToken(plaintext string) (string, error) {
	key := deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// OpenToken decrypts a base64 AES-GCM sealed token.
func OpenToken(sealed string) (string, error) {
	key := deriveKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("sealed token too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// SaveToken encrypts and stores the token under dir/tokens/jwt.enc (0600).
func SaveToken(dir, plaintext string) error {
	if err := os.MkdirAll(filepath.Join(dir, "tokens"), 0o700); err != nil {
		return err
	}
	sealed, err := SealToken(plaintext)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tokens", "jwt.enc"), []byte(sealed), 0o600)
}

// LoadToken reads the stored token (decrypted). Returns "" if absent.
func LoadToken(dir string) (string, error) {
	p := filepath.Join(dir, "tokens", "jwt.enc")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return OpenToken(strings.TrimSpace(string(data)))
}

// ResolveToken selects a token by priority (SKILLS §12.2 S2):
// env OPENSTRATA_TOKEN > stored file > explicit flag.
func ResolveToken(dir, flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	if v := os.Getenv("OPENSTRATA_TOKEN"); v != "" {
		return v
	}
	if v, _ := LoadToken(dir); v != "" {
		return v
	}
	return ""
}

// GoVersion is surfaced by `aictl version` for build provenance.
func GoVersion() string { return runtime.Version() }
