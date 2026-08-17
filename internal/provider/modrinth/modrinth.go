// Package modrinth implements the provider.Provider interface against
// the Modrinth Labrinth API (https://docs.modrinth.com/).
package modrinth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ryan3311/modship/internal/model"
)

const apiBase = "https://api.modrinth.com/v2"

// Provider implements provider.Provider for Modrinth.
type Provider struct {
	http *http.Client
}

// New creates a Modrinth provider. No API key needed for read operations.
func New() *Provider {
	return &Provider{
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() model.Provider { return model.ProviderModrinth }

// mrSearchHit is a single result from the Modrinth search endpoint.
type mrSearchHit struct {
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Description string `json:"description"`
	IconURL   string `json:"icon_url"`
	Downloads int    `json:"downloads"`
}

// mrSearchResponse is the search endpoint response.
type mrSearchResponse struct {
	Hits []mrSearchHit `json:"hits"`
}

// mrVersion is a Modrinth project version.
type mrVersion struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	DatePublished string   `json:"date_published"`
	Files         []mrFile `json:"files"`
}

// mrFile is a downloadable file within a Modrinth version.
type mrFile struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

func (p *Provider) do(ctx context.Context, path string, q url.Values) (*http.Response, error) {
	u := apiBase + path
	if q != nil {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("modrinth: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	return p.http.Do(req)
}

// Search returns modpacks matching the query.
func (p *Provider) Search(ctx context.Context, query string) ([]model.Modpack, error) {
	q := url.Values{}
	q.Set("query", query)
	q.Set("facets", `[["project_type:modpack"]]`)
	resp, err := p.do(ctx, "/search", q)
	if err != nil {
		return nil, fmt.Errorf("modrinth: search %q: %w", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("modrinth: search %q: %s", query, resp.Status)
	}
	var r mrSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("modrinth: decode search: %w", err)
	}
	out := make([]model.Modpack, 0, len(r.Hits))
	for _, h := range r.Hits {
		out = append(out, model.Modpack{
			ID:        h.ProjectID,
			Name:      h.Title,
			Slug:      h.Slug,
			Summary:   h.Description,
			Thumbnail: h.IconURL,
			Provider:  model.ProviderModrinth,
			Downloads: h.Downloads,
		})
	}
	return out, nil
}

// GetVersions returns all available versions for a modpack (by slug or ID).
func (p *Provider) GetVersions(ctx context.Context, slug string) ([]model.Version, error) {
	resp, err := p.do(ctx, fmt.Sprintf("/project/%s/version", url.PathEscape(slug)), nil)
	if err != nil {
		return nil, fmt.Errorf("modrinth: get versions for %q: %w", slug, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("modrinth: get versions for %q: %s", slug, resp.Status)
	}
	var versions []mrVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("modrinth: decode versions: %w", err)
	}
	out := make([]model.Version, 0, len(versions))
	for _, v := range versions {
		gameVer := ""
		if len(v.GameVersions) > 0 {
			gameVer = v.GameVersions[0]
		}
		released, _ := time.Parse(time.RFC3339, v.DatePublished)
		ver := model.Version{
			ID:          v.ID,
			Name:        v.Name,
			GameVersion: gameVer,
			Loaders:     v.Loaders,
			Released:    released,
		}
		if len(v.Files) > 0 {
			ver.DownloadURL = v.Files[0].URL
			ver.Filename = v.Files[0].Filename
		}
		out = append(out, ver)
	}
	return out, nil
}

// ResolveVersion returns a specific version by its version ID.
func (p *Provider) ResolveVersion(ctx context.Context, slug, versionID string) (*model.Version, error) {
	resp, err := p.do(ctx, fmt.Sprintf("/version/%s", url.PathEscape(versionID)), nil)
	if err != nil {
		return nil, fmt.Errorf("modrinth: get version %s: %w", versionID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("modrinth: get version %s: %s", versionID, resp.Status)
	}
	var v mrVersion
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("modrinth: decode version: %w", err)
	}
	gameVer := ""
	if len(v.GameVersions) > 0 {
		gameVer = v.GameVersions[0]
	}
	released, _ := time.Parse(time.RFC3339, v.DatePublished)
	ver := model.Version{
		ID:          v.ID,
		Name:        v.Name,
		GameVersion: gameVer,
		Loaders:     v.Loaders,
		Released:    released,
	}
	if len(v.Files) > 0 {
		ver.DownloadURL = v.Files[0].URL
		ver.Filename = v.Files[0].Filename
	}
	return &ver, nil
}
