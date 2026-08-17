package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ryan3311/modship/internal/cloudflare"
	"github.com/ryan3311/modship/internal/compose"
	"github.com/ryan3311/modship/internal/config"
	"github.com/ryan3311/modship/internal/deploy"
	"github.com/ryan3311/modship/internal/provider"
	"github.com/ryan3311/modship/internal/provider/curseforge"
	"github.com/ryan3311/modship/internal/provider/modrinth"
	"github.com/ryan3311/modship/internal/router"
	"github.com/ryan3311/modship/internal/store"
	"github.com/ryan3311/modship/internal/web"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatalf("modship: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Open the SQLite store.
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer db.Close()

	// Register modpack providers.
	registry := provider.NewRegistry()
	registry.Register(curseforge.New(cfg.CurseForgeAPIKey))
	registry.Register(modrinth.New())

	// Create integration clients.
	routerClient := router.New(cfg.RouterURL)
	var cfClient *cloudflare.Client
	if cfg.CloudflareAPIKey != "" && cfg.CloudflareZoneID != "" {
		cfClient = cloudflare.New(cfg.CloudflareAPIKey, cfg.CloudflareZoneID)
	}

	// Compose generator.
	composeGen := compose.New(cfg.DockerNetwork, cfg.CurseForgeAPIKey)

	// Host IP for DNS records (auto-detect or from env).
	hostIP := os.Getenv("MODSHIP_HOST_IP")

	// Deployment manager.
	dm := deploy.New(db, composeGen, routerClient, cfClient, cfg.DataDir, cfg.BaseDomain, hostIP)

	// Web server.
	srv := web.New(db, registry, dm)
	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Handler(),
	}

	// Start HTTP server in background.
	go func() {
		log.Printf("modship listening on %s", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("modship shutting down...")
	return httpServer.Shutdown(context.Background())
}
