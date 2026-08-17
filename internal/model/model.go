package model

import "time"

// Provider is the modpack platform that hosts the modpack.
type Provider string

const (
	ProviderCurseForge Provider = "curseforge"
	ProviderModrinth   Provider = "modrinth"
)

// ServerState represents the lifecycle state of a deployed server.
type ServerState string

const (
	StateRunning  ServerState = "running"
	StateStopped  ServerState = "stopped"
	StateError    ServerState = "error"
	StateCreating ServerState = "creating"
	StateDeleting ServerState = "deleting"
)

// Modpack is a modpack returned by a provider search.
type Modpack struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Summary     string `json:"summary"`
	Thumbnail   string `json:"thumbnail"`
	Provider    Provider `json:"provider"`
	Downloads   int    `json:"downloads"`
}

// Version is a specific release of a modpack.
type Version struct {
	ID        string `json:"id"`        // file ID (CurseForge) or version ID (Modrinth)
	Name      string `json:"name"`      // display name
	GameVersion string `json:"game_version"` // e.g. "1.20.1"
	Loaders   []string `json:"loaders"` // e.g. ["forge"], ["fabric"]
	Released  time.Time `json:"released"`
	DownloadURL string `json:"download_url"`
	Filename  string `json:"filename"`
}

// Server is a deployed minecraft server managed by modship.
type Server struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`            // container + compose project name
	Provider         Provider    `json:"provider"`
	ModpackSlug      string      `json:"modpack_slug"`
	ModpackName      string      `json:"modpack_name"`
	FileID           string      `json:"file_id"`         // CF file ID or Modrinth version ID
	GameVersion      string      `json:"game_version"`
	JavaImage        string      `json:"java_image"`      // e.g. "java17", "java21"
	Memory           string      `json:"memory"`          // e.g. "10G"
	Domain           string      `json:"domain"`          // e.g. "create-delight.max4king.com"
	CloudflareRecordID string    `json:"cloudflare_record_id"`
	ContainerID      string      `json:"container_id"`
	ComposePath      string      `json:"compose_path"`
	State            ServerState `json:"state"`
	Whitelist        []string    `json:"whitelist"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}
