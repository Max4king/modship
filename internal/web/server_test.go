package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ryan3311/modship/internal/model"
	"github.com/ryan3311/modship/internal/provider"
	"github.com/ryan3311/modship/internal/store"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name     model.Provider
	modpacks []model.Modpack
	versions []model.Version
}

func (m *mockProvider) Name() model.Provider { return m.name }
func (m *mockProvider) Search(ctx context.Context, q string) ([]model.Modpack, error) {
	return m.modpacks, nil
}
func (m *mockProvider) GetVersions(ctx context.Context, slug string) ([]model.Version, error) {
	return m.versions, nil
}
func (m *mockProvider) ResolveVersion(ctx context.Context, slug, versionID string) (*model.Version, error) {
	if len(m.versions) > 0 {
		return &m.versions[0], nil
	}
	return &model.Version{ID: versionID}, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	reg := provider.NewRegistry()
	reg.Register(&mockProvider{
		name: model.ProviderModrinth,
		modpacks: []model.Modpack{
			{ID: "p1", Name: "Test Pack", Slug: "test-pack", Summary: "A test", Provider: model.ProviderModrinth},
		},
		versions: []model.Version{
			{ID: "v1", Name: "1.0.0", GameVersion: "1.20.1"},
		},
	})

	return New(s, reg, nil) // deploy manager nil — we test handlers that don't need it
}

func TestHandleIndex(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !contains(body, "modship") {
		t.Error("index page should contain 'modship'")
	}
	if !contains(body, "No servers deployed yet") {
		t.Error("index should show empty state")
	}
}

func TestHandleIndex_WithServers(t *testing.T) {
	srv := newTestServer(t)
	// Insert a server into the store.
	srv.store.CreateServer(context.Background(), &model.Server{
		ID:          "s1",
		Name:        "my-server",
		Provider:    model.ProviderModrinth,
		ModpackSlug: "test-pack",
		ModpackName: "Test Pack",
		FileID:      "v1",
		GameVersion: "1.20.1",
		JavaImage:   "java17",
		Memory:      "10G",
		Domain:      "my-server.example.com",
		State:       model.StateRunning,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !contains(body, "my-server") {
		t.Error("index should list the server")
	}
	if !contains(body, "Test Pack") {
		t.Error("index should show modpack name")
	}
}

func TestHandleSearch(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&provider=modrinth", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var results []model.Modpack
	json.NewDecoder(rec.Body).Decode(&results)
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].Name != "Test Pack" {
		t.Errorf("Name = %q, want Test Pack", results[0].Name)
	}
}

func TestHandleSearch_MissingQuery(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleVersions(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/versions?slug=test-pack&provider=modrinth", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var versions []model.Version
	json.NewDecoder(rec.Body).Decode(&versions)
	if len(versions) != 1 {
		t.Fatalf("len = %d, want 1", len(versions))
	}
	if versions[0].GameVersion != "1.20.1" {
		t.Errorf("GameVersion = %q, want 1.20.1", versions[0].GameVersion)
	}
}

func TestHandleVersions_MissingSlug(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/versions", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleGetServer_NotFound(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/servers/nonexistent", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
