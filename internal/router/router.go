// Package router provides a client for the itzg/mc-router HTTP API
// (https://github.com/itzg/mc-router) to manage hostname→backend mappings
// at runtime without restarting the router container.
package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the mc-router HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a router client. baseURL is the mc-router API root
// (e.g. "http://localhost:25566").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// AddRoute registers a hostname→backend mapping in mc-router.
// backend is typically "<container-name>:25565".
func (c *Client) AddRoute(ctx context.Context, host, backend string) error {
	body, err := json.Marshal(map[string]string{
		"host":    host,
		"backend": backend,
	})
	if err != nil {
		return fmt.Errorf("router: marshal route: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/routes", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("router: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("router: add route %q: %w", host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("router: add route %q: %s: %s", host, resp.Status, string(b))
	}
	return nil
}

// RemoveRoute deletes a hostname mapping from mc-router.
func (c *Client) RemoveRoute(ctx context.Context, host string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		c.baseURL+"/routes/"+url.PathEscape(host), nil)
	if err != nil {
		return fmt.Errorf("router: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("router: remove route %q: %w", host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("router: remove route %q: %s: %s", host, resp.Status, string(b))
	}
	return nil
}

// ListRoutes returns all current hostname→backend mappings.
func (c *Client) ListRoutes(ctx context.Context) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/routes", nil)
	if err != nil {
		return nil, fmt.Errorf("router: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("router: list routes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("router: list routes: %s: %s", resp.Status, string(b))
	}
	var routes map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("router: decode routes: %w", err)
	}
	return routes, nil
}
