package curseforge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	p := New("test-cf-key")
	p.http = &http.Client{Transport: &mockTransport{handler: handler}}
	return p
}

func TestName(t *testing.T) {
	p := New("key")
	if p.Name() != model.ProviderCurseForge {
		t.Errorf("Name = %q, want %q", p.Name(), model.ProviderCurseForge)
	}
}

func TestSearch_Success(t *testing.T) {
	var gotFilter, gotAPIKey string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotFilter = req.URL.Query().Get("searchFilter")
		gotAPIKey = req.Header.Get("x-api-key")
		json.NewEncoder(w).Encode(cfResponse[[]cfMod]{
			Data: []cfMod{
				{ID: 42, Name: "Test Pack", Slug: "test-pack", Summary: "A test", DownloadCount: 1000,
					Logo: struct {
						ThumbnailURL string `json:"thumbnailUrl"`
					}{ThumbnailURL: "https://thumb.url/icon.png"}},
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
	if results[0].ID != "42" {
		t.Errorf("ID = %q, want 42", results[0].ID)
	}
	if results[0].Name != "Test Pack" {
		t.Errorf("Name = %q, want Test Pack", results[0].Name)
	}
	if results[0].Slug != "test-pack" {
		t.Errorf("Slug = %q, want test-pack", results[0].Slug)
	}
	if results[0].Thumbnail != "https://thumb.url/icon.png" {
		t.Errorf("Thumbnail = %q", results[0].Thumbnail)
	}
	if results[0].Downloads != 1000 {
		t.Errorf("Downloads = %d, want 1000", results[0].Downloads)
	}
	if gotFilter != "test query" {
		t.Errorf("searchFilter = %q, want 'test query'", gotFilter)
	}
	if gotAPIKey != "test-cf-key" {
		t.Errorf("api key header = %q, want test-cf-key", gotAPIKey)
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
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("slug") != "" {
			// resolveModID search — return one result
			json.NewEncoder(w).Encode(cfResponse[[]cfMod]{
				Data: []cfMod{{ID: 42, Slug: "test-pack"}},
			})
		} else {
			// files endpoint — return files
			json.NewEncoder(w).Encode(cfResponse[[]cfFile]{
				Data: []cfFile{
					{ID: 100, DisplayName: "v1.0", FileName: "test.zip", DownloadURL: "https://dl.url/test.zip",
						GameVersions: []string{"1.20.1"}, FileDate: "2024-01-15T00:00:00Z"},
				},
			})
		}
	})
	versions, err := p.GetVersions(context.Background(), "test-pack")
	if err != nil {
		t.Fatalf("GetVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("len = %d, want 1", len(versions))
	}
	if versions[0].ID != "100" {
		t.Errorf("ID = %q, want 100", versions[0].ID)
	}
	if versions[0].Name != "v1.0" {
		t.Errorf("Name = %q, want v1.0", versions[0].Name)
	}
	if versions[0].GameVersion != "1.20.1" {
		t.Errorf("GameVersion = %q, want 1.20.1", versions[0].GameVersion)
	}
	if versions[0].DownloadURL != "https://dl.url/test.zip" {
		t.Errorf("DownloadURL = %q", versions[0].DownloadURL)
	}
}

func TestGetVersions_NumericModID(t *testing.T) {
	var gotPath string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		json.NewEncoder(w).Encode(cfResponse[[]cfFile]{Data: []cfFile{}})
	})
	_, err := p.GetVersions(context.Background(), "42")
	if err != nil {
		t.Fatalf("GetVersions: %v", err)
	}
	if gotPath != "/v1/mods/42/files" {
		t.Errorf("path = %q, want /v1/mods/42/files", gotPath)
	}
}

func TestResolveVersion_Success(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(cfResponse[cfFile]{
			Data: cfFile{ID: 100, DisplayName: "v1.0", FileName: "test.zip",
				DownloadURL: "https://dl.url/test.zip", GameVersions: []string{"1.20.1"}},
		})
	})
	v, err := p.ResolveVersion(context.Background(), "42", strconv.Itoa(100))
	if err != nil {
		t.Fatalf("ResolveVersion: %v", err)
	}
	if v.ID != "100" {
		t.Errorf("ID = %q, want 100", v.ID)
	}
	if v.GameVersion != "1.20.1" {
		t.Errorf("GameVersion = %q, want 1.20.1", v.GameVersion)
	}
}
