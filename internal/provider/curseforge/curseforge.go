// Package curseforge implements the provider.Provider interface against
// the CurseForge Core API (https://docs.curseforge.com/).
package curseforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ryan3311/modship/internal/model"
)

const apiBase = "https://api.curseforge.com/v1"

// Provider implements provider.Provider for CurseForge.
type Provider struct {
	apiKey string
	http   *http.Client
}

// New creates a CurseForge provider. apiKey is the CurseForge Core API key.
func New(apiKey string) *Provider {
	return &Provider{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() model.Provider { return model.ProviderCurseForge }

// cfResponse is the standard CurseForge API envelope.
type cfResponse[T any] struct {
	Data T `json:"data"`
}

// cfMod is the CurseForge mod search result.
type cfMod struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Summary       string `json:"summary"`
	DownloadCount int    `json:"downloadCount"`
	Logo          struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"logo"`
}

// cfFile is a CurseForge mod file.
type cfFile struct {
	ID           int      `json:"id"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	FileDate     string   `json:"fileDate"`
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersions"`
}

func (p *Provider) do(ctx context.Context, path string, q url.Values) (*http.Response, error) {
	if p.apiKey == "" {
		return nil, errors.New("curseforge: API key not set — set CF_API_KEY in your .env file or environment. Get one at https://console.curseforge.com/")
	}
	u := apiBase + path
	if q != nil {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("curseforge: build request: %w", err)
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("Accept", "application/json")
	return p.http.Do(req)
}

// statusError maps a non-2xx response to a user-friendly error, calling out
// the most common CurseForge authentication failures explicitly.
func statusError(what string, resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusForbidden:
		return errors.New("curseforge: API key is invalid or expired (403 Forbidden). Check your CF_API_KEY value. Get one at https://console.curseforge.com/")
	case http.StatusUnauthorized:
		return errors.New("curseforge: unauthorized (401) — CF_API_KEY may be missing or invalid")
	default:
		return fmt.Errorf("curseforge: %s: %s", what, resp.Status)
	}
}

// Search returns modpacks matching the query, sorted by popularity.
// page is 0-indexed; pageSize is capped at 50 (the CurseForge API limit).
func (p *Provider) Search(ctx context.Context, query string, page, pageSize int) ([]model.Modpack, error) {
	if page < 0 {
		page = 0
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}
	q := url.Values{}
	q.Set("gameId", "432")   // Minecraft
	q.Set("classId", "4471") // Modpacks
	q.Set("searchFilter", query)
	q.Set("sortField", "2") // Total downloads (popularity)
	q.Set("sortOrder", "desc")
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("index", strconv.Itoa(page*pageSize)) // CurseForge uses an offset, not a page number
	resp, err := p.do(ctx, "/mods/search", q)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, statusError(fmt.Sprintf("search %q", query), resp)
	}
	var r cfResponse[[]cfMod]
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("curseforge: decode search: %w", err)
	}
	out := make([]model.Modpack, 0, len(r.Data))
	for _, m := range r.Data {
		out = append(out, model.Modpack{
			ID:        strconv.Itoa(m.ID),
			Name:      m.Name,
			Slug:      m.Slug,
			Summary:   m.Summary,
			Thumbnail: m.Logo.ThumbnailURL,
			Provider:  model.ProviderCurseForge,
			Downloads: m.DownloadCount,
		})
	}
	return out, nil
}

// GetVersions returns all available files for a modpack identified by its
// CurseForge mod ID. Since the API requires the numeric mod ID (not slug),
// the slug is first resolved via search.
func (p *Provider) GetVersions(ctx context.Context, slug string) ([]model.Version, error) {
	modID, err := p.resolveModID(ctx, slug)
	if err != nil {
		return nil, err
	}
	resp, err := p.do(ctx, fmt.Sprintf("/mods/%d/files", modID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, statusError(fmt.Sprintf("get files for %q", slug), resp)
	}
	var r cfResponse[[]cfFile]
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("curseforge: decode files: %w", err)
	}
	out := make([]model.Version, 0, len(r.Data))
	for _, f := range r.Data {
		gameVer := ""
		if len(f.GameVersions) > 0 {
			gameVer = f.GameVersions[0]
		}
		released, _ := time.Parse(time.RFC3339, f.FileDate)
		out = append(out, model.Version{
			ID:          strconv.Itoa(f.ID),
			Name:        f.DisplayName,
			GameVersion: gameVer,
			Released:    released,
			DownloadURL: f.DownloadURL,
			Filename:    f.FileName,
		})
	}
	return out, nil
}

// ResolveVersion returns a specific file by its ID (as string).
func (p *Provider) ResolveVersion(ctx context.Context, slug, versionID string) (*model.Version, error) {
	modID, err := p.resolveModID(ctx, slug)
	if err != nil {
		return nil, err
	}
	fileID, err := strconv.Atoi(versionID)
	if err != nil {
		return nil, fmt.Errorf("curseforge: invalid file ID %q: %w", versionID, err)
	}
	resp, err := p.do(ctx, fmt.Sprintf("/mods/%d/files/%d", modID, fileID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, statusError(fmt.Sprintf("get file %s", versionID), resp)
	}
	var r cfResponse[cfFile]
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("curseforge: decode file: %w", err)
	}
	gameVer := ""
	if len(r.Data.GameVersions) > 0 {
		gameVer = r.Data.GameVersions[0]
	}
	released, _ := time.Parse(time.RFC3339, r.Data.FileDate)
	return &model.Version{
		ID:          strconv.Itoa(r.Data.ID),
		Name:        r.Data.DisplayName,
		GameVersion: gameVer,
		Released:    released,
		DownloadURL: r.Data.DownloadURL,
		Filename:    r.Data.FileName,
	}, nil
}

// resolveModID resolves a slug to its numeric CurseForge mod ID via search.
func (p *Provider) resolveModID(ctx context.Context, slug string) (int, error) {
	// Try as a numeric ID first.
	if id, err := strconv.Atoi(slug); err == nil {
		return id, nil
	}
	// Otherwise search by slug.
	q := url.Values{}
	q.Set("gameId", "432")
	q.Set("classId", "4471")
	q.Set("searchFilter", slug)
	q.Set("slug", slug)
	resp, err := p.do(ctx, "/mods/search", q)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, statusError(fmt.Sprintf("resolve slug %q", slug), resp)
	}
	var r cfResponse[[]cfMod]
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, fmt.Errorf("curseforge: decode slug search: %w", err)
	}
	for _, m := range r.Data {
		if m.Slug == slug {
			return m.ID, nil
		}
	}
	if len(r.Data) > 0 {
		return r.Data[0].ID, nil
	}
	return 0, fmt.Errorf("curseforge: modpack %q not found", slug)
}
