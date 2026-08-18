package curseforge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	var gotFilter, gotAPIKey, gotSortField, gotSortOrder, gotPageSize, gotIndex string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotFilter = req.URL.Query().Get("searchFilter")
		gotAPIKey = req.Header.Get("x-api-key")
		gotSortField = req.URL.Query().Get("sortField")
		gotSortOrder = req.URL.Query().Get("sortOrder")
		gotPageSize = req.URL.Query().Get("pageSize")
		gotIndex = req.URL.Query().Get("index")
		json.NewEncoder(w).Encode(cfResponse[[]cfMod]{
			Data: []cfMod{
				{ID: 42, Name: "Test Pack", Slug: "test-pack", Summary: "A test", DownloadCount: 1000,
					Logo: struct {
						ThumbnailURL string `json:"thumbnailUrl"`
					}{ThumbnailURL: "https://thumb.url/icon.png"}},
			},
		})
	})
	results, err := p.Search(context.Background(), "test query", 0, 10)
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
	if gotSortField != "2" {
		t.Errorf("sortField = %q, want \"2\" (popularity)", gotSortField)
	}
	if gotSortOrder != "desc" {
		t.Errorf("sortOrder = %q, want desc", gotSortOrder)
	}
	if gotPageSize != "10" {
		t.Errorf("pageSize = %q, want 10", gotPageSize)
	}
	if gotIndex != "0" {
		t.Errorf("index = %q, want 0", gotIndex)
	}
}

func TestSearch_PaginationParams(t *testing.T) {
	var gotPageSize, gotIndex string
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		gotPageSize = req.URL.Query().Get("pageSize")
		gotIndex = req.URL.Query().Get("index")
		json.NewEncoder(w).Encode(cfResponse[[]cfMod]{Data: []cfMod{}})
	})
	if _, err := p.Search(context.Background(), "test", 1, 25); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotPageSize != "25" {
		t.Errorf("pageSize = %q, want 25", gotPageSize)
	}
	if gotIndex != "25" {
		t.Errorf("index = %q, want 25 (page 1 * 25 items)", gotIndex)
	}
}

func TestSearch_Error(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := p.Search(context.Background(), "test", 0, 10)
	if err == nil {
		t.Fatal("Search should error on 500")
	}
	if !strings.Contains(err.Error(), "500 Internal Server Error") {
		t.Errorf("error should include the status code, got: %s", err.Error())
	}
}

func TestSearch_EmptyAPIKey(t *testing.T) {
	called := false
	p := New("")
	p.http = &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, req *http.Request) {
		called = true
	}}}
	_, err := p.Search(context.Background(), "test", 0, 10)
	if err == nil {
		t.Fatal("Search should error when API key is empty")
	}
	if called {
		t.Fatal("no API request should be made when API key is empty")
	}
	want := "curseforge: API key not set — set CF_API_KEY in your .env file or environment. Get one at https://console.curseforge.com/"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestGetVersions_EmptyAPIKey(t *testing.T) {
	called := false
	p := New("")
	p.http = &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, req *http.Request) {
		called = true
	}}}
	// "42" is a numeric mod ID, so no slug-resolution search is issued;
	// do() is hit directly on the files endpoint.
	_, err := p.GetVersions(context.Background(), "42")
	if err == nil {
		t.Fatal("GetVersions should error when API key is empty")
	}
	if called {
		t.Fatal("no API request should be made when API key is empty")
	}
	want := "curseforge: API key not set — set CF_API_KEY in your .env file or environment. Get one at https://console.curseforge.com/"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestSearch_Forbidden(t *testing.T) {
	p := New("bad-key")
	p.http = &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}}}
	_, err := p.Search(context.Background(), "test", 0, 10)
	if err == nil {
		t.Fatal("Search should error on 403")
	}
	want := "curseforge: API key is invalid or expired (403 Forbidden). Check your CF_API_KEY value. Get one at https://console.curseforge.com/"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestSearch_Unauthorized(t *testing.T) {
	p := New("bad-key")
	p.http = &http.Client{Transport: &mockTransport{handler: func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}}}
	_, err := p.Search(context.Background(), "test", 0, 10)
	if err == nil {
		t.Fatal("Search should error on 401")
	}
	want := "curseforge: unauthorized (401) — CF_API_KEY may be missing or invalid"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
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
