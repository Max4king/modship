package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all runtime configuration for modship.
type Config struct {
	// DataDir is where generated compose files and server data live.
	DataDir string
	// DBPath is the SQLite database path.
	DBPath string
	// ListenAddr is the HTTP server listen address.
	ListenAddr string

	// CurseForge API key (required for CurseForge provider).
	CurseForgeAPIKey string
	// Cloudflare API token (required for DNS management).
	CloudflareAPIKey string
	// CloudflareZoneID is the DNS zone to manage records in.
	CloudflareZoneID string
	// BaseDomain is the parent domain (e.g. "max4king.com").
	BaseDomain string

	// RouterURL is the mc-router HTTP API base URL.
	RouterURL string
	// RouterHostPort is the public-facing port mc-router listens on.
	RouterHostPort string

	// DockerNetwork is the name of the shared minecraft network.
	DockerNetwork string

	// DefaultWhitelist is the list of MC usernames added to every new server.
	DefaultWhitelist []string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	dataDir := envOr("MODSHIP_DATA_DIR", "./deployments")
	cfg := &Config{
		DataDir:           dataDir,
		DBPath:            envOr("MODSHIP_DB_PATH", filepath.Join(dataDir, "modship.db")),
		ListenAddr:        envOr("MODSHIP_LISTEN", ":8080"),
		CurseForgeAPIKey:  os.Getenv("CF_API_KEY"),
		CloudflareAPIKey:  os.Getenv("CLOUDFLARE_API_KEY"),
		CloudflareZoneID:  os.Getenv("CLOUDFLARE_ZONE_ID"),
		BaseDomain:        envOr("MODSHIP_BASE_DOMAIN", "max4king.com"),
		RouterURL:         envOr("MODSHIP_ROUTER_URL", "http://localhost:25566"),
		RouterHostPort:   envOr("MODSHIP_ROUTER_PORT", "25565"),
		DockerNetwork:    envOr("MODSHIP_DOCKER_NETWORK", "modship_minecraft-network"),
		DefaultWhitelist: []string{"Ryan_3311", "AyumiKa", "MeTamTam"},
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
