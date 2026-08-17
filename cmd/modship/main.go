package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatalf("modship: %v", err)
	}
}

func run(ctx context.Context) error {
	// TODO: wire config, store, providers, router, cloudflare, web server.
	fmt.Println("modship starting...")
	<-ctx.Done()
	fmt.Println("modship shutting down")
	return nil
}
