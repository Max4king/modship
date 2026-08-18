package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvOr(t *testing.T) {
	t.Run("returns env when set", func(t *testing.T) {
		t.Setenv("TEST_ENV_OR", "value")
		if got := envOr("TEST_ENV_OR", "fallback"); got != "value" {
			t.Errorf("envOr = %q, want %q", got, "value")
		}
	})
	t.Run("returns fallback when not set", func(t *testing.T) {
		if got := envOr("TEST_ENV_OR_MISSING", "fallback"); got != "fallback" {
			t.Errorf("envOr = %q, want %q", got, "fallback")
		}
	})
	t.Run("returns fallback when empty", func(t *testing.T) {
		t.Setenv("TEST_ENV_OR_EMPTY", "")
		if got := envOr("TEST_ENV_OR_EMPTY", "fallback"); got != "fallback" {
			t.Errorf("envOr = %q, want %q", got, "fallback")
		}
	})
}

func TestLoadDotEnv(t *testing.T) {
	t.Run("returns nil for missing file", func(t *testing.T) {
		if err := loadDotEnv("/nonexistent/.env"); err != nil {
			t.Errorf("loadDotEnv missing file: got %v, want nil", err)
		}
	})
	t.Run("loads key=value pairs", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		os.WriteFile(path, []byte("TEST_DOTENV_KEY=hello world\n"), 0o644)

		t.Setenv("TEST_DOTENV_KEY", "") // ensure not set
		os.Unsetenv("TEST_DOTENV_KEY")

		if err := loadDotEnv(path); err != nil {
			t.Fatalf("loadDotEnv: %v", err)
		}
		if got := os.Getenv("TEST_DOTENV_KEY"); got != "hello world" {
			t.Errorf("TEST_DOTENV_KEY = %q, want %q", got, "hello world")
		}
	})
	t.Run("ignores comments and blanks", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		os.WriteFile(path, []byte("# comment\n\n  \nREAL_KEY=val\n"), 0o644)

		os.Unsetenv("REAL_KEY")
		if err := loadDotEnv(path); err != nil {
			t.Fatalf("loadDotEnv: %v", err)
		}
		if got := os.Getenv("REAL_KEY"); got != "val" {
			t.Errorf("REAL_KEY = %q, want %q", got, "val")
		}
	})
	t.Run("strips surrounding quotes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		os.WriteFile(path, []byte(`QUOTED="quoted_val"`+"\n"), 0o644)

		os.Unsetenv("QUOTED")
		if err := loadDotEnv(path); err != nil {
			t.Fatalf("loadDotEnv: %v", err)
		}
		if got := os.Getenv("QUOTED"); got != "quoted_val" {
			t.Errorf("QUOTED = %q, want %q", got, "quoted_val")
		}
	})
	t.Run("does not override existing env", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")
		os.WriteFile(path, []byte("EXISTING_KEY=from_file\n"), 0o644)

		t.Setenv("EXISTING_KEY", "from_env")
		if err := loadDotEnv(path); err != nil {
			t.Fatalf("loadDotEnv: %v", err)
		}
		if got := os.Getenv("EXISTING_KEY"); got != "from_env" {
			t.Errorf("EXISTING_KEY = %q, want %q (env should win)", got, "from_env")
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		// Clear relevant env vars
		for _, k := range []string{"CF_API_KEY", "CLOUDFLARE_API_KEY", "CLOUDFLARE_ZONE_ID",
			"MODSHIP_DATA_DIR", "MODSHIP_DB_PATH", "MODSHIP_LISTEN", "MODSHIP_BASE_DOMAIN"} {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}

		dir := t.TempDir()
		t.Setenv("MODSHIP_DATA_DIR", dir)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.DataDir != dir {
			t.Errorf("DataDir = %q, want %q", cfg.DataDir, dir)
		}
		if cfg.ListenAddr == "" {
			t.Error("ListenAddr should have a default")
		}
		if cfg.CurseForgeAPIKey != "" {
			t.Errorf("CurseForgeAPIKey = %q, want empty", cfg.CurseForgeAPIKey)
		}
	})
	t.Run("reads from env", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MODSHIP_DATA_DIR", dir)
		t.Setenv("CF_API_KEY", "test-cf-key")
		t.Setenv("MODSHIP_LISTEN", ":9090")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.CurseForgeAPIKey != "test-cf-key" {
			t.Errorf("CurseForgeAPIKey = %q, want %q", cfg.CurseForgeAPIKey, "test-cf-key")
		}
		if cfg.ListenAddr != ":9090" {
			t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
		}
	})
}
