// Package cloudflare provides a minimal client for the Cloudflare API v4
// to manage DNS A records for modship server domains.
package cloudflare

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

// Client manages DNS records in a single Cloudflare zone.
type Client struct {
	apiKey string
	zoneID string
	http   *http.Client
}

// New creates a Cloudflare DNS client.
func New(apiKey, zoneID string) *Client {
	return &Client{
		apiKey: apiKey,
		zoneID: zoneID,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

const apiBase = "https://api.cloudflare.com/client/v4"

// cfResponse is the standard Cloudflare API envelope.
type cfResponse[T any] struct {
	Success bool            `json:"success"`
	Errors  []cfError       `json:"errors"`
	Result  T               `json:"result"`
}

type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type dnsRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	u := apiBase + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("cloudflare: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("cloudflare: request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

func unwrapError(status int, data []byte) error {
	var r cfResponse[json.RawMessage]
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("cloudflare: %d: %s", status, string(data))
	}
	if r.Success {
		return nil
	}
	if len(r.Errors) > 0 {
		return fmt.Errorf("cloudflare: %d: %s", status, r.Errors[0].Message)
	}
	return fmt.Errorf("cloudflare: %d: %s", status, string(data))
}

// CreateRecord creates an A record and returns the new record ID.
func (c *Client) CreateRecord(ctx context.Context, name, content string) (string, error) {
	body, _ := json.Marshal(dnsRecord{
		Type:    "A",
		Name:    name,
		Content: content,
		Proxied: false,
	})
	path := fmt.Sprintf("/zones/%s/dns_records", c.zoneID)
	data, status, err := c.do(ctx, http.MethodPost, path, body)
	if err != nil {
		return "", err
	}
	if err := unwrapError(status, data); err != nil {
		return "", err
	}
	var r cfResponse[dnsRecord]
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("cloudflare: decode create response: %w", err)
	}
	return r.Result.ID, nil
}

// DeleteRecord removes a DNS record by its ID.
func (c *Client) DeleteRecord(ctx context.Context, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", c.zoneID, url.PathEscape(recordID))
	data, status, err := c.do(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil // already gone — treat as success
	}
	return unwrapError(status, data)
}

// FindRecord returns the record ID for the given DNS name, or "" if not found.
func (c *Client) FindRecord(ctx context.Context, name string) (string, error) {
	path := fmt.Sprintf("/zones/%s/dns_records?name=%s", c.zoneID, url.QueryEscape(name))
	data, status, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	if err := unwrapError(status, data); err != nil {
		return "", err
	}
	var r cfResponse[[]dnsRecord]
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("cloudflare: decode list response: %w", err)
	}
	if len(r.Result) == 0 {
		return "", nil
	}
	return r.Result[0].ID, nil
}

// EnsureRecord creates the A record if it doesn't already exist.
// Returns the record ID (existing or newly created).
func (c *Client) EnsureRecord(ctx context.Context, name, content string) (string, error) {
	if id, err := c.FindRecord(ctx, name); err != nil {
		return "", err
	} else if id != "" {
		return id, nil
	}
	return c.CreateRecord(ctx, name, content)
}

// Ensure strings import is used (for TrimRight in future extensions).
var _ = strings.TrimSpace
