// Package deploy orchestrates the full server deployment lifecycle:
// compose generation → docker compose up → mc-router route → cloudflare DNS → store.
package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/ryan3311/modship/internal/compose"
	"github.com/ryan3311/modship/internal/model"
	"github.com/ryan3311/modship/internal/store"
	"github.com/ryan3311/modship/internal/cloudflare"
	"github.com/ryan3311/modship/internal/router"
)

// Manager coordinates the deployment lifecycle for minecraft servers.
type Manager struct {
	store     *store.Store
	compose   *compose.Generator
	router    *router.Client
	cloudflare *cloudflare.Client
	dataDir   string
	baseDomain string
	hostIP    string
}

// New creates a deployment manager.
func New(s *store.Store, g *compose.Generator, r *router.Client, c *cloudflare.Client, dataDir, baseDomain, hostIP string) *Manager {
	return &Manager{
		store:      s,
		compose:    g,
		router:     r,
		cloudflare: c,
		dataDir:    dataDir,
		baseDomain: baseDomain,
		hostIP:     hostIP,
	}
}

// DeployRequest is the input for creating a new server deployment.
type DeployRequest struct {
	Name        string         `json:"name"`
	Provider    model.Provider `json:"provider"`
	ModpackSlug string         `json:"modpack_slug"`
	ModpackName string         `json:"modpack_name"`
	FileID      string         `json:"file_id"`
	GameVersion string         `json:"game_version"`
	JavaImage   string         `json:"java_image"`
	Memory      string         `json:"memory"`
	Whitelist   []string       `json:"whitelist"`
}

// Deploy creates a new server: generates compose, starts containers,
// registers the mc-router mapping, creates the DNS record, and persists state.
func (m *Manager) Deploy(ctx context.Context, req *DeployRequest) (*model.Server, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("deploy: name is required")
	}
	if req.GameVersion == "" {
		return nil, fmt.Errorf("deploy: game_version is required")
	}
	if req.JavaImage == "" {
		req.JavaImage = "java17"
	}
	if req.Memory == "" {
		req.Memory = "10G"
	}

	// Check for name collision.
	if existing, err := m.store.GetServerByName(ctx, req.Name); err == nil && existing != nil {
		return nil, fmt.Errorf("deploy: server %q already exists", req.Name)
	}

	domain := fmt.Sprintf("%s.%s", req.Name, m.baseDomain)
	serverDir := filepath.Join(m.dataDir, req.Name)

	srv := &model.Server{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Provider:    req.Provider,
		ModpackSlug: req.ModpackSlug,
		ModpackName: req.ModpackName,
		FileID:      req.FileID,
		GameVersion: req.GameVersion,
		JavaImage:   req.JavaImage,
		Memory:      req.Memory,
		Domain:      domain,
		State:       model.StateCreating,
		Whitelist:   req.Whitelist,
	}

	// 1. Generate and write compose file.
	composePath, err := m.compose.Write(srv, serverDir)
	if err != nil {
		return nil, fmt.Errorf("deploy: generate compose: %w", err)
	}
	srv.ComposePath = composePath

	// 2. Persist initial state.
	if err := m.store.CreateServer(ctx, srv); err != nil {
		_ = os.RemoveAll(serverDir)
		return nil, fmt.Errorf("deploy: store server: %w", err)
	}

	// 3. docker compose up -d
	if err := m.composeUp(ctx, serverDir); err != nil {
		srv.State = model.StateError
		_ = m.store.UpdateServer(ctx, srv)
		return nil, fmt.Errorf("deploy: compose up: %w", err)
	}

	// 4. Register route with mc-router.
	backend := fmt.Sprintf("%s:25565", req.Name)
	if err := m.router.AddRoute(ctx, domain, backend); err != nil {
		// Non-fatal: server is running, just not routable.
		fmt.Fprintf(os.Stderr, "warn: router add route: %v\n", err)
	}

	// 5. Create Cloudflare DNS record.
	if m.hostIP != "" && m.cloudflare != nil {
		recordID, err := m.cloudflare.EnsureRecord(ctx, domain, m.hostIP)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: cloudflare create record: %v\n", err)
		} else {
			srv.CloudflareRecordID = recordID
		}
	}

	srv.State = model.StateRunning
	if err := m.store.UpdateServer(ctx, srv); err != nil {
		return nil, fmt.Errorf("deploy: update server state: %w", err)
	}

	return srv, nil
}

// Start starts a stopped server.
func (m *Manager) Start(ctx context.Context, id string) error {
	srv, err := m.store.GetServer(ctx, id)
	if err != nil {
		return fmt.Errorf("start: get server: %w", err)
	}
	serverDir := filepath.Dir(srv.ComposePath)
	if err := m.composeUp(ctx, serverDir); err != nil {
		return fmt.Errorf("start: compose up: %w", err)
	}
	srv.State = model.StateRunning
	return m.store.UpdateServer(ctx, srv)
}

// Stop stops a running server without removing it.
func (m *Manager) Stop(ctx context.Context, id string) error {
	srv, err := m.store.GetServer(ctx, id)
	if err != nil {
		return fmt.Errorf("stop: get server: %w", err)
	}
	serverDir := filepath.Dir(srv.ComposePath)
	if err := m.composeStop(ctx, serverDir); err != nil {
		return fmt.Errorf("stop: compose stop: %w", err)
	}
	srv.State = model.StateStopped
	return m.store.UpdateServer(ctx, srv)
}

// Delete removes a server entirely: stops containers, removes route,
// deletes DNS record, removes files, deletes DB row.
func (m *Manager) Delete(ctx context.Context, id string) error {
	srv, err := m.store.GetServer(ctx, id)
	if err != nil {
		return fmt.Errorf("delete: get server: %w", err)
	}

	srv.State = model.StateDeleting
	_ = m.store.UpdateServer(ctx, srv)

	serverDir := filepath.Dir(srv.ComposePath)

	// 1. Stop and remove containers.
	_ = m.composeDown(ctx, serverDir)

	// 2. Remove mc-router mapping.
	if srv.Domain != "" {
		if err := m.router.RemoveRoute(ctx, srv.Domain); err != nil {
			fmt.Fprintf(os.Stderr, "warn: router remove route: %v\n", err)
		}
	}

	// 3. Delete Cloudflare DNS record.
	if srv.CloudflareRecordID != "" && m.cloudflare != nil {
		if err := m.cloudflare.DeleteRecord(ctx, srv.CloudflareRecordID); err != nil {
			fmt.Fprintf(os.Stderr, "warn: cloudflare delete record: %v\n", err)
		}
	}

	// 4. Remove files.
	_ = os.RemoveAll(serverDir)

	// 5. Delete DB row.
	return m.store.DeleteServer(ctx, id)
}

// composeUp runs `docker compose -p <name> up -d` in the given directory.
func (m *Manager) composeUp(ctx context.Context, dir string) error {
	return m.runCompose(ctx, dir, "up", "-d")
}

// composeStop runs `docker compose -p <name> stop`.
func (m *Manager) composeStop(ctx context.Context, dir string) error {
	return m.runCompose(ctx, dir, "stop")
}

// composeDown runs `docker compose -p <name> down -v`.
func (m *Manager) composeDown(ctx context.Context, dir string) error {
	return m.runCompose(ctx, dir, "down", "-v")
}

func (m *Manager) runCompose(ctx context.Context, dir string, args ...string) error {
	name := filepath.Base(dir)
	fullArgs := append([]string{"compose", "-p", name, "-f", filepath.Join(dir, "docker-compose.yml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
