package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withEnv temporarily sets env vars for the duration of the test and
// restores them afterwards. The map's keys are env-var names; "" values
// unset the variable. We use this instead of t.Setenv only for the
// "GOOGLE_CLOUD_*" cleanup pattern where an outer shell might already
// have those set.
func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

// clearAllEnv unsets every env var Load() consults so a test starts
// from a clean baseline regardless of the developer's shell.
func clearAllEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ASK_GEMINI_PROJECT", "ASK_GEMINI_LOCATION",
		"ASK_GEMINI_MODEL", "ASK_GEMINI_REQUEST_TIMEOUT",
		"GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_LOCATION",
	} {
		t.Setenv(k, "")
	}
}

func TestLoad_MissingProject_IsError(t *testing.T) {
	clearAllEnv(t)
	// No config file, no env: Load() must reject.
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.toml"))
	if err == nil {
		t.Fatal("expected error for missing project, got nil")
	}
	if !strings.Contains(err.Error(), "GCP project") {
		t.Fatalf("expected GCP-project error, got: %v", err)
	}
}

func TestLoad_EnvOnly_TakesAskGeminiOverGoogleCloud(t *testing.T) {
	clearAllEnv(t)
	withEnv(t, map[string]string{
		"ASK_GEMINI_PROJECT":   "ask-wins",
		"GOOGLE_CLOUD_PROJECT": "google-loses",
	})

	cfg, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GCP.Project != "ask-wins" {
		t.Fatalf("project: want ask-wins, got %q", cfg.GCP.Project)
	}
	if cfg.GCP.Location != DefaultLocation {
		t.Fatalf("location default: want %q, got %q", DefaultLocation, cfg.GCP.Location)
	}
	if cfg.Model.Name != DefaultModel {
		t.Fatalf("model default: want %q, got %q", DefaultModel, cfg.Model.Name)
	}
}

func TestLoad_FileThenEnv(t *testing.T) {
	clearAllEnv(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[gcp]
project = "file-project"
location = "asia-northeast1"

[model]
name = "gemini-2.5-pro"
request_timeout = 300
`), 0o600); err != nil {
		t.Fatal(err)
	}

	// File-only path.
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GCP.Project != "file-project" {
		t.Fatalf("project: want file-project, got %q", cfg.GCP.Project)
	}
	if cfg.GCP.Location != "asia-northeast1" {
		t.Fatalf("location: want asia-northeast1, got %q", cfg.GCP.Location)
	}
	if cfg.Model.Name != "gemini-2.5-pro" {
		t.Fatalf("model: want gemini-2.5-pro, got %q", cfg.Model.Name)
	}
	if cfg.Model.RequestTimeout != 300 {
		t.Fatalf("timeout: want 300, got %d", cfg.Model.RequestTimeout)
	}

	// Env override on top of file.
	withEnv(t, map[string]string{
		"ASK_GEMINI_PROJECT":  "env-project",
		"ASK_GEMINI_LOCATION": "us-east1",
	})
	cfg2, err := Load(path)
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}
	if cfg2.GCP.Project != "env-project" {
		t.Fatalf("env override project: want env-project, got %q", cfg2.GCP.Project)
	}
	if cfg2.GCP.Location != "us-east1" {
		t.Fatalf("env override location: want us-east1, got %q", cfg2.GCP.Location)
	}
	// File values for non-overridden fields survive.
	if cfg2.Model.Name != "gemini-2.5-pro" {
		t.Fatalf("model retained: want gemini-2.5-pro, got %q", cfg2.Model.Name)
	}
}

func TestLoad_StrictDecode_RejectsUnknownField(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("ASK_GEMINI_PROJECT", "any") // satisfy the project requirement

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
[gcp]
project = "p"

[model]
name = "m"
typoooo_field = "would-silently-fail-non-strict"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected strict-decode error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown fields") {
		t.Fatalf("expected unknown-fields error, got: %v", err)
	}
}

func TestParseIntEnv_BadValueLeavesDefault(t *testing.T) {
	clearAllEnv(t)
	t.Setenv("ASK_GEMINI_PROJECT", "p")
	t.Setenv("ASK_GEMINI_REQUEST_TIMEOUT", "not-a-number")

	cfg, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model.RequestTimeout != DefaultRequestTimeout {
		t.Fatalf("timeout: want default %d, got %d",
			DefaultRequestTimeout, cfg.Model.RequestTimeout)
	}
}
