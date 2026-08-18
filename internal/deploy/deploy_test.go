package deploy

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ryan3311/modship/internal/cloudflare"
	"github.com/ryan3311/modship/internal/compose"
	"github.com/ryan3311/modship/internal/model"
	"github.com/ryan3311/modship/internal/router"
	"github.com/ryan3311/modship/internal/store"
)

func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	g := compose.New("modship_minecraft-network", "test-cf-key")
	r := router.New("http://localhost:25566") // won't be called in validation tests
	var c *cloudflare.Client // nil — won't be called in validation tests

	dm := New(s, g, r, c, dir, "example.com", "")
	return dm, s
}

func TestDeploy_MissingName(t *testing.T) {
	dm, _ := newTestManager(t)
	_, err := dm.Deploy(context.Background(), &DeployRequest{
		Provider:    model.ProviderCurseForge,
		ModpackSlug: "test",
		FileID:      "123",
		GameVersion: "1.20.1",
	})
	if err == nil {
		t.Error("Deploy should error with missing name")
	}
}

func TestDeploy_MissingGameVersion(t *testing.T) {
	dm, _ := newTestManager(t)
	_, err := dm.Deploy(context.Background(), &DeployRequest{
		Name:        "test-server",
		Provider:    model.ProviderCurseForge,
		ModpackSlug: "test",
		FileID:      "123",
	})
	if err == nil {
		t.Error("Deploy should error with missing game_version")
	}
}

func TestDeploy_DefaultsFilled(t *testing.T) {
	dm, _ := newTestManager(t)
	// This will fail at docker compose up, but we can test that defaults
	// are applied before that by checking the store entry.
	req := &DeployRequest{
		Name:        "test-srv",
		Provider:    model.ProviderModrinth,
		ModpackSlug: "test-pack",
		ModpackName: "Test Pack",
		FileID:      "ver-1",
		GameVersion: "1.20.1",
	}
	_, err := dm.Deploy(context.Background(), req)
	// Will error because docker compose isn't available, but store entry
	// should be created with defaults.
	if err == nil {
		// If docker IS available (unlikely in test env), verify defaults anyway.
		srv, _ := dm.store.GetServerByName(context.Background(), "test-srv")
		if srv != nil {
			if srv.JavaImage != "java17" {
				t.Errorf("JavaImage = %q, want java17", srv.JavaImage)
			}
			if srv.Memory != "10G" {
				t.Errorf("Memory = %q, want 10G", srv.Memory)
			}
		}
		return
	}
	// Docker not available — verify the server was still stored with defaults.
	srv, _ := dm.store.GetServerByName(context.Background(), "test-srv")
	if srv == nil {
		t.Fatal("server should be in store even if deploy failed")
	}
	if srv.JavaImage != "java17" {
		t.Errorf("JavaImage = %q, want java17", srv.JavaImage)
	}
	if srv.Memory != "10G" {
		t.Errorf("Memory = %q, want 10G", srv.Memory)
	}
	if srv.Domain != "test-srv.example.com" {
		t.Errorf("Domain = %q, want test-srv.example.com", srv.Domain)
	}
	if srv.State != model.StateError {
		t.Errorf("State = %q, want %q", srv.State, model.StateError)
	}
}

func TestDeploy_NameCollision(t *testing.T) {
	dm, s := newTestManager(t)
	// Insert an existing server with the same name.
	existing := &model.Server{
		ID:          "existing-id",
		Name:        "my-server",
		Provider:    model.ProviderCurseForge,
		ModpackSlug: "pack",
		FileID:      "123",
		GameVersion: "1.20.1",
		JavaImage:   "java17",
		Memory:      "10G",
		State:       model.StateRunning,
	}
	if err := s.CreateServer(context.Background(), existing); err != nil {
		t.Fatalf("seed existing: %v", err)
	}
	_, err := dm.Deploy(context.Background(), &DeployRequest{
		Name:        "my-server",
		Provider:    model.ProviderCurseForge,
		ModpackSlug: "other-pack",
		FileID:      "456",
		GameVersion: "1.20.1",
	})
	if err == nil {
		t.Error("Deploy should error with name collision")
	}
}

func TestStart_UnknownServer(t *testing.T) {
	dm, _ := newTestManager(t)
	err := dm.Start(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("Start should error for unknown server")
	}
}

func TestStop_UnknownServer(t *testing.T) {
	dm, _ := newTestManager(t)
	err := dm.Stop(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("Stop should error for unknown server")
	}
}

func TestDelete_UnknownServer(t *testing.T) {
	dm, _ := newTestManager(t)
	err := dm.Delete(context.Background(), "nonexistent-id")
	if err == nil {
		t.Error("Delete should error for unknown server")
	}
}
