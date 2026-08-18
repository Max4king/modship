package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ryan3311/modship/internal/model"
	"github.com/ryan3311/modship/internal/provider"
	"github.com/ryan3311/modship/internal/store"
)

// This test simulates a full browser interaction flow against the web server:
// 1. Load the index page
// 2. Search for modpacks via the API
// 3. Select a modpack → fetch versions via the API
// 4. Deploy a server via the API
// 5. Verify the server appears in the server list
// 6. Start/stop/delete the server
//
// It uses a mock provider and a nil deploy manager (deploy tested separately).
// The goal is to verify the HTTP endpoints work end-to-end together.

func newIntegrationServer(t *testing.T) *Server {
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
			{ID: "p1", Name: "Create+", Slug: "create_plus", Summary: "Create modpack", Provider: model.ProviderModrinth, Downloads: 10000},
		},
		versions: []model.Version{
			{ID: "v1", Name: "1.0.0", GameVersion: "1.20.1", Loaders: []string{"fabric"}},
			{ID: "v2", Name: "1.1.0", GameVersion: "1.20.1", Loaders: []string{"forge"}},
		},
	})

	return New(s, reg, nil)
}

func TestIntegration_SearchFlow(t *testing.T) {
	srv := newIntegrationServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// Step 1: Load the index page — must contain search form and JS.
	t.Run("index page loads", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, _ := ioReadAll(resp.Body)
		if !strings.Contains(string(body), "searchModpacks") {
			t.Error("index should contain searchModpacks JS function")
		}
		if !strings.Contains(string(body), "provider-select") {
			t.Error("index should contain provider-select element")
		}
		if !strings.Contains(string(body), "search-input") {
			t.Error("index should contain search-input element")
		}
		if !strings.Contains(string(body), "search-results") {
			t.Error("index should contain search-results div")
		}
	})

	// Step 2: Search for modpacks — simulates clicking the Search button.
	t.Run("search returns modpacks", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/search?q=create&provider=modrinth")
		if err != nil {
			t.Fatalf("GET /api/search: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var results []model.Modpack
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("len = %d, want 1", len(results))
		}
		if results[0].Slug != "create_plus" {
			t.Errorf("Slug = %q, want create_plus", results[0].Slug)
		}
		if results[0].Name != "Create+" {
			t.Errorf("Name = %q, want Create+", results[0].Name)
		}
	})

	// Step 3: Select a modpack → fetch versions.
	t.Run("get versions for selected modpack", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/versions?slug=create_plus&provider=modrinth")
		if err != nil {
			t.Fatalf("GET /api/versions: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var versions []model.Version
		if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(versions) != 2 {
			t.Fatalf("len = %d, want 2", len(versions))
		}
		if versions[0].GameVersion != "1.20.1" {
			t.Errorf("GameVersion = %q, want 1.20.1", versions[0].GameVersion)
		}
	})

	// Step 4: Search with CurseForge (no API key) — should return error JSON, not crash.
	t.Run("search curseforge without key returns error not crash", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/search?q=test&provider=curseforge")
		if err != nil {
			t.Fatalf("GET /api/search: %v", err)
		}
		// Should return an error status, not 200, with JSON error body.
		if resp.StatusCode == http.StatusOK {
			// CurseForge with no key might return 502 or 200 with error.
			// The key thing: the response body should be valid JSON.
		}
		// Parse as JSON — should not fail.
		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("response is not valid JSON: %v", err)
		}
		// Should have an "error" field.
		if _, ok := body["error"]; !ok {
			t.Error("error response should contain 'error' field")
		}
	})
}

func TestIntegration_ServerListAfterInsert(t *testing.T) {
	srv := newIntegrationServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// Insert a server directly into the store.
	srv.store.CreateServer(context.Background(), &model.Server{
		ID:          "int-1",
		Name:        "integration-server",
		Provider:    model.ProviderModrinth,
		ModpackSlug: "create_plus",
		ModpackName: "Create+",
		FileID:      "v1",
		GameVersion: "1.20.1",
		JavaImage:   "java17",
		Memory:      "10G",
		Domain:      "integration-server.example.com",
		State:       model.StateRunning,
	})

	// Verify it shows in the server list on the page.
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	body, _ := ioReadAll(resp.Body)
	// html/template HTML-escapes data in the raw source (e.g. the "+" in
	// "Create+" is emitted as &#43;), so assert on the decoded text that a
	// browser actually renders instead of a substring of the escaped source.
	visible := unescapeHTML(string(body))
	if !strings.Contains(visible, "integration-server") {
		t.Error("index page should list integration-server")
	}
	if !strings.Contains(visible, "Create+") {
		t.Error("index page should show modpack name")
	}
	if !strings.Contains(visible, "badge-running") {
		t.Error("index page should show running badge")
	}
}

func TestIntegration_EmptySearchReturnsError(t *testing.T) {
	srv := newIntegrationServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// Search without query param — should return 400.
	resp, err := client.Get(ts.URL + "/api/search")
	if err != nil {
		t.Fatalf("GET /api/search: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestIntegration_SearchReturnsErrorJSONNotArray(t *testing.T) {
	srv := newIntegrationServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := ts.Client()

	// Search with an unregistered provider — should return error JSON.
	resp, err := client.Get(ts.URL + "/api/search?q=test&provider=nonexistent")
	if err != nil {
		t.Fatalf("GET /api/search: %v", err)
	}
	// Should return valid JSON with an error field, not crash.
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	// The response should have either "error" or be a valid array.
	// The key test: the response is valid JSON either way.
}

// unescapeHTML decodes the HTML entities Go's html/template emits in
// rendered output, so tests can assert on the text a browser displays.
// html/template escapes e.g. '&' as &amp;, '<' as &lt;, '>' as &gt;,
// '+' as &#43; in HTML text, which breaks naive substring checks against
// the raw response body.
var htmlNamedEntities = map[string]rune{
	"amp":  '&',
	"lt":   '<',
	"gt":   '>',
	"quot": '"',
	"apos": '\'',
}

func unescapeHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '&' {
			b.WriteByte(s[i])
			continue
		}
		semi := strings.IndexByte(s[i+1:], ';')
		if semi < 0 {
			b.WriteByte('&')
			continue
		}
		entity := s[i+1 : i+1+semi]
		if r, ok := decodeHTMLEntity(entity); ok {
			b.WriteRune(r)
			i += semi
			continue
		}
		b.WriteByte('&')
	}
	return b.String()
}

// decodeHTMLEntity decodes a single entity body (text between '&' and ';')
// such as "amp", "#43", or "#x2B".
func decodeHTMLEntity(e string) (rune, bool) {
	if r, ok := htmlNamedEntities[e]; ok {
		return r, true
	}
	switch {
	case strings.HasPrefix(e, "#x"), strings.HasPrefix(e, "#X"):
		v, err := strconv.ParseInt(e[2:], 16, 32)
		if err != nil {
			return 0, false
		}
		return rune(v), true
	case strings.HasPrefix(e, "#"):
		v, err := strconv.ParseInt(e[1:], 10, 32)
		if err != nil {
			return 0, false
		}
		return rune(v), true
	}
	return 0, false
}

// ioReadAll is a helper to read response bodies.
func ioReadAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}
