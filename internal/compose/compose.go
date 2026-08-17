// Package compose generates per-server docker-compose.yml files for
// modship deployments. Each server is its own compose project, deployed
// with `docker compose -p <name> up -d`.
package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ryan3311/modship/internal/model"
)

// Generator produces docker-compose.yml content for a modship server.
type Generator struct {
	dockerNetwork string
	cfAPIKey      string
}

// New creates a compose generator.
// dockerNetwork is the name of the shared external minecraft network.
// cfAPIKey is the CurseForge API key (injected into CF deployments).
func New(dockerNetwork, cfAPIKey string) *Generator {
	return &Generator{
		dockerNetwork: dockerNetwork,
		cfAPIKey:       cfAPIKey,
	}
}

const composeTpl = `services:
  {{.Name}}:
    image: ghcr.io/itzg/minecraft-server:{{.JavaImage}}
    tty: true
    stdin_open: true
    environment:
      EULA: "TRUE"
      MEMORY: "{{.Memory}}"
      VERSION: "{{.GameVersion}}"
      MAX_TICK_TIME: "-1"
      WHITELIST: |
{{- range .Whitelist}}
        {{.}}
{{- end}}
{{.ProviderEnv}}    restart: always
    volumes:
      - ./data:/data
    networks:
      - minecraft-network
networks:
  minecraft-network:
    external: true
    name: {{.DockerNetwork}}
`

type tplData struct {
	Name          string
	JavaImage     string
	Memory        string
	GameVersion   string
	Whitelist     []string
	ProviderEnv   string
	DockerNetwork string
}

// Generate returns the docker-compose.yml content for the given server.
func (g *Generator) Generate(srv *model.Server) (string, error) {
	providerEnv, err := g.providerEnv(srv)
	if err != nil {
		return "", err
	}
	data := tplData{
		Name:          srv.Name,
		JavaImage:     srv.JavaImage,
		Memory:        srv.Memory,
		GameVersion:   srv.GameVersion,
		Whitelist:     srv.Whitelist,
		ProviderEnv:   providerEnv,
		DockerNetwork: g.dockerNetwork,
	}
	t, err := template.New("compose").Parse(composeTpl)
	if err != nil {
		return "", fmt.Errorf("compose: parse template: %w", err)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("compose: execute template: %w", err)
	}
	return buf.String(), nil
}

// providerEnv returns the provider-specific environment block for the compose.
func (g *Generator) providerEnv(srv *model.Server) (string, error) {
	switch srv.Provider {
	case model.ProviderCurseForge:
		return fmt.Sprintf(`      MODPACK_PLATFORM: AUTO_CURSEFORGE
      CF_API_KEY: %s
      CF_SLUG: %q
      CF_FILE_ID: %q
`, g.cfAPIKey, srv.ModpackSlug, srv.FileID), nil

	case model.ProviderModrinth:
		return fmt.Sprintf(`      MODPACK_PLATFORM: MODRINTH
      MODRINTH_MODPACK: %q
      MODRINTH_VERSION: %q
`, srv.ModpackSlug, srv.FileID), nil

	default:
		return "", fmt.Errorf("compose: unknown provider %q", srv.Provider)
	}
}

// Write generates the compose file and writes it to dir/docker-compose.yml.
// Returns the full path to the written file.
func (g *Generator) Write(srv *model.Server, dir string) (string, error) {
	content, err := g.Generate(srv)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("compose: create dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("compose: write %s: %w", path, err)
	}
	return path, nil
}
