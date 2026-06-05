// Package config loads ask-gemini-mcp configuration from a TOML file
// with environment-variable overrides.
//
// Schema mirrors the rest of the gem-* tools in the util-series:
// [gcp] (project / location) + [model] (name / request_timeout).
// Precedence:
//
//	ASK_GEMINI_* env  >  GOOGLE_CLOUD_* env  >  config file  >  built-in defaults
//
// The default config path is ~/.config/ask-gemini-mcp/config.toml.
// Pass an explicit path to Load(), or pass "" to use the default.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Built-in defaults applied when neither the config file nor an env var
// supplies a value. Kept as exported constants so tests can reference
// them by name.
const (
	DefaultLocation       = "us-central1"
	DefaultModel          = "gemini-2.5-flash"
	DefaultRequestTimeout = 180
)

// Config is the root configuration. It is populated by Load() from the
// on-disk TOML, then merged with env-var overrides.
type Config struct {
	GCP   GCPConfig   `toml:"gcp"`
	Model ModelConfig `toml:"model"`
}

// GCPConfig holds the Google Cloud project / region that the Vertex AI
// client connects to. Project is required at runtime; Location falls
// back to DefaultLocation.
type GCPConfig struct {
	Project  string `toml:"project"`
	Location string `toml:"location"`
}

// ModelConfig holds the Gemini model name and per-request timeout.
// Other model knobs (temperature, top-p, etc.) are intentionally
// absent — ask-gemini-mcp is a transparent consultation channel and
// the caller's prompt drives behaviour, not server-side tuning.
type ModelConfig struct {
	Name           string `toml:"name"`
	RequestTimeout int    `toml:"request_timeout"`
}

// Load reads ask-gemini-mcp's configuration. If path is empty, the
// default path under ~/.config/ask-gemini-mcp/config.toml is used; if
// no file exists at that path, the function silently proceeds with
// built-in defaults so a fresh install can still run when
// ASK_GEMINI_PROJECT (or GOOGLE_CLOUD_PROJECT) is exported.
//
// TOML decoding is strict: any unknown field in the file is a hard
// error so silent typos fail fast (feedback_strict_json_decode.md).
//
// Env-var overrides are applied AFTER the TOML decode so they always
// take precedence. A missing project ID after all resolution sources
// is treated as a hard error — the binary cannot reach Vertex AI
// without one.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path == "" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, ".config", "ask-gemini-mcp", "config.toml")
		}
	}
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			meta, err := toml.DecodeFile(path, cfg)
			if err != nil {
				return nil, fmt.Errorf("parse config %s: %w", path, err)
			}
			if undecoded := meta.Undecoded(); len(undecoded) > 0 {
				return nil, fmt.Errorf("parse config %s: unknown fields: %v", path, undecoded)
			}
		}
	}

	applyEnvOverrides(cfg)

	if cfg.GCP.Project == "" {
		return nil, fmt.Errorf("GCP project is required: set [gcp].project in config or ASK_GEMINI_PROJECT / GOOGLE_CLOUD_PROJECT env var")
	}

	return cfg, nil
}

// defaults returns a Config populated with built-in values. Kept as a
// constructor (not a package-level var) so tests can always start from
// a fresh, mutation-free baseline.
func defaults() *Config {
	return &Config{
		GCP: GCPConfig{
			Location: DefaultLocation,
		},
		Model: ModelConfig{
			Name:           DefaultModel,
			RequestTimeout: DefaultRequestTimeout,
		},
	}
}

// applyEnvOverrides walks the env-var precedence table from the RFP.
// Tool-specific variables (ASK_GEMINI_*) always win over the generic
// GCP fallbacks; numeric inputs that fail to parse fall through so a
// malformed env var never silently zero-fills a production setting.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ASK_GEMINI_PROJECT"); v != "" {
		cfg.GCP.Project = v
	} else if v := os.Getenv("GOOGLE_CLOUD_PROJECT"); v != "" {
		cfg.GCP.Project = v
	}

	if v := os.Getenv("ASK_GEMINI_LOCATION"); v != "" {
		cfg.GCP.Location = v
	} else if v := os.Getenv("GOOGLE_CLOUD_LOCATION"); v != "" {
		cfg.GCP.Location = v
	}

	if v := os.Getenv("ASK_GEMINI_MODEL"); v != "" {
		cfg.Model.Name = v
	}

	parseIntEnv("ASK_GEMINI_REQUEST_TIMEOUT", &cfg.Model.RequestTimeout)
}

// parseIntEnv reads name as an integer and writes it into *dst when
// the env var is set AND parses cleanly. A blank env var or a
// non-integer value leaves *dst untouched.
func parseIntEnv(name string, dst *int) {
	v := os.Getenv(name)
	if v == "" {
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return
	}
	*dst = n
}
