// Package provider defines the abstraction over modpack platforms
// (CurseForge, Modrinth). Implementations live under sub-packages.
package provider

import (
	"context"

	"github.com/ryan3311/modship/internal/model"
)

// Provider is the interface every modpack platform must implement.
// It handles search and version resolution only — the itzg/minecraft-server
// Docker image performs the actual modpack download and install via env vars.
type Provider interface {
	// Name returns the provider identifier (e.g. "curseforge", "modrinth").
	Name() model.Provider

	// Search returns modpacks matching the query.
	Search(ctx context.Context, query string) ([]model.Modpack, error)

	// GetVersions returns all available versions/releases for a modpack slug.
	GetVersions(ctx context.Context, slug string) ([]model.Version, error)

	// ResolveVersion returns a specific version by its ID (file ID for
	// CurseForge, version ID for Modrinth).
	ResolveVersion(ctx context.Context, slug, versionID string) (*model.Version, error)
}

// Registry holds providers keyed by their Name().
type Registry struct {
	providers map[model.Provider]Provider
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[model.Provider]Provider)}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get returns the provider for the given name, or nil if not registered.
func (r *Registry) Get(name model.Provider) Provider {
	return r.providers[name]
}

// All returns all registered providers.
func (r *Registry) All() []Provider {
	out := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	return out
}
