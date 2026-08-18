package modrinth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ryan3311/modship/internal/model"
)

type mockTransport struct {
	handler http.HandlerFunc
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	m.handler(rec, req)
	return rec.Result(), nil
}

func newTestProvider(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	p := New()
	p.http = &http.Client{Transport: &mockTransport{handler: handler}}
	return p
}

func TestName(t *testing.T) {
	p := New()
	if p.Name() != model.ProviderModrinth {
		t.Errorf("Name = %q, want %q", p.Name(), model.ProviderModrinth)
	}
}

func TestSearch_Success(t *testing.T) {
	var gotQuery, gotFacets string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotQuery = req.URL.Query().Get("query")
		gotFacets = req.URL.Query().Get("facets")
		json.NewEncoder(w).Encode(mrSearchResponse{
			Hits: []mrSearchHit{
				{ProjectID: "abc123", Title: "Test Pack", Slug: "test-pack",
					Description: "A test", IconURL: "https://icon.url/img.png", Downloads: 5000},
			},
		})
	})
	results, err := p.Search(context.Background(), "test query")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len = %d, want 1", len(results))
	}
	if results[0].ID != "abc123" {
		t.Errorf("ID = %q, want abc123", results[0].ID)
	}
	if results[0].Name != "Test Pack" {
		t.Errorf("Name = %q, want Test Pack", results[0].Name)
	}
	if results[0].Slug != "test-pack" {
		t.Errorf("Slug = %q, want test-pack", results[0].Slug)
	}
	if results[0].Thumbnail != "https://icon.url/img.png" {
		t.Errorf("Thumbnail = %q", results[0].Thumbnail)
	}
	if results[0].Downloads != 5000 {
		t.Errorf("Downloads = %d, want 5000", results[0].Downloads)
	}
	if gotQuery != "test query" {
		t.Errorf("query = %q, want 'test query'", gotQuery)
	}
	if gotFacets == "" {
		t.Error("facets should be set for modpack search")
	}
}

func TestSearch_Error(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := p.Search(context.Background(), "test")
	if err == nil {
		t.Error("Search should error on 500")
	}
}

func TestGetVersions_Success(t *testing.T) {
	var gotPath string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		json.NewEncoder(w).Encode([]mrVersion{
			{ID: "ver-1", Name: "1.0.0", GameVersions: []string{"1.20.1"}, Loaders: []string{"forge"},
				DatePublished: "2024-01-15T00:00:00Z",
				Files: []mrFile{{URL: "https://dl.url/file.zip", Filename: "test.mrpack"}}},
		})
	})
	versions, err := p.GetVersions(context.Background(), "test-pack")
	if err != nil {
		t.Fatalf("GetVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("len = %d, want 1", len(versions))
	}
	if versions[0].ID != "ver-1" {
		t.Errorf("ID = %q, want ver-1", versions[0].ID)
	}
	if versions[0].GameVersion != "1.20.1" {
		t.Errorf("GameVersion = %q, want 1.20.1", versions[0].GameVersion)
	}
	if len(versions[0].Loaders) != 1 || versions[0].Loaders[0] != "forge" {
		t.Errorf("Loaders = %v, want [forge]", versions[0].Loaders)
	}
	if versions[0].DownloadURL != "https://dl.url/file.zip" {
		t.Errorf("DownloadURL = %q", versions[0].DownloadURL)
	}
	if versions[0].Filename != "test.mrpack" {
		t.Errorf("Filename = %q, want test.mrpack", versions[0].Filename)
	}
	if gotPath != "/v2/project/test-pack/version" {
		t.Errorf("path = %q, want /v2/project/test-pack/version", gotPath)
	}
}

func TestGetVersions_Error(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := p.GetVersions(context.Background(), "missing")
	if err == nil {
		t.Error("GetVersions should error on 404")
	}
}

func TestResolveVersion_Success(t *testing.T) {
	var gotPath string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		json.NewEncoder(w).Encode(mrVersion{
			ID: "ver-1", Name: "1.0.0", GameVersions: []string{"1.20.1"}, Loaders: []string{"fabric"},
			Files: []mrFile{{URL: "https://dl.url/file.zip", Filename: "test.mrpack"}},
		})
	})
	v, err := p.ResolveVersion(context.Background(), "test-pack", "ver-1")
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if v.ID != "ver-1" {
		t.Errorf("ID = %q, want ver-1", v.ID)
	}
	if v.GameVersion != "1.20.1" {
		t.Errorf("GameVersion = %q, want 1.20.1", v.GameVersion)
	}
	if len(v.Loaders) != 1 || v.Loaders[0] != "fabric" {
		t.Errorf("Loaders = %v, want [fabric]", v.Loaders)
	}
	if gotPath != "/v2/version/ver-1" {
		t.Errorf("path = %q, want /v2/version/ver-1", gotPath)
	}
}
