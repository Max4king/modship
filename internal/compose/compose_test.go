package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryan3311/modship/internal/model"
)

func sampleServer(provider model.Provider) *model.Server {
	return &model.Server{
		ID:          "test-id",
		Name:        "myserver",
		Provider:    provider,
		ModpackSlug: "test-pack",
		ModpackName: "Test Pack",
		FileID:      "12345",
		GameVersion: "1.20.1",
		JavaImage:   "java17",
		Memory:      "10G",
		Whitelist:   []string{"Player1", "Player2"},
	}
}

func TestGenerate_CurseForge(t *testing.T) {
	g := New("modship_minecraft-network", "test-cf-key")
	yaml, err := g.Generate(sampleServer(model.ProviderCurseForge))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checks := []string{
		"myserver:",
		"ghcr.io/itzg/minecraft-server:java17",
		`MEMORY: "10G"`,
		`VERSION: "1.20.1"`,
		`MAX_TICK_TIME: "-1"`,
		"Player1",
		"Player2",
		"MODPACK_PLATFORM: AUTO_CURSEFORGE",
		"CF_API_KEY: test-cf-key",
		`CF_SLUG: "test-pack"`,
		`CF_FILE_ID: "12345"`,
		"modship_minecraft-network",
	}
	for _, want := range checks {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n--- YAML ---\n%s", want, yaml)
		}
	}
}

func TestGenerate_Modrinth(t *testing.T) {
	g := New("modship_minecraft-network", "test-cf-key")
	yaml, err := g.Generate(sampleServer(model.ProviderModrinth))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	checks := []string{
		"MODPACK_PLATFORM: MODRINTH",
		`MODRINTH_MODPACK: "test-pack"`,
		`MODRINTH_VERSION: "12345"`,
	}
	for _, want := range checks {
		if !strings.Contains(yaml, want) {
			t.Errorf("YAML missing %q\n--- YAML ---\n%s", want, yaml)
		}
	}
}

func TestGenerate_UnknownProvider(t *testing.T) {
	g := New("net", "key")
	_, err := g.Generate(&model.Server{Provider: "bogus"})
	if err == nil {
		t.Error("Generate should error for unknown provider")
	}
}

func TestWrite(t *testing.T) {
	g := New("modship_minecraft-network", "test-key")
	dir := filepath.Join(t.TempDir(), "myserver")
	srv := sampleServer(model.ProviderCurseForge)

	path, err := g.Write(srv, dir)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if path != filepath.Join(dir, "docker-compose.yml") {
		t.Errorf("path = %q, want %q", path, filepath.Join(dir, "docker-compose.yml"))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "myserver:") {
		t.Errorf("written file missing expected content")
	}
}
