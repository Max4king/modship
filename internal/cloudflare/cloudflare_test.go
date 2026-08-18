package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockTransport intercepts HTTP requests regardless of URL, routing them
// to a test handler. This works around the hardcoded apiBase constant.
type mockTransport struct {
	handler http.HandlerFunc
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	m.handler(rec, req)
	return rec.Result(), nil
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	c := New("test-key", "test-zone")
	c.http = &http.Client{Transport: &mockTransport{handler: handler}}
	return c
}

func TestCreateRecord_Success(t *testing.T) {
	var gotPath, gotMethod string
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		gotMethod = req.Method
		json.NewEncoder(w).Encode(cfResponse[dnsRecord]{
			Success: true,
			Result:  dnsRecord{ID: "rec-123"},
		})
	})
	id, err := c.CreateRecord(context.Background(), "test.example.com", "1.2.3.4")
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if id != "rec-123" {
		t.Errorf("id = %q, want rec-123", id)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/client/v4/zones/test-zone/dns_records" {
		t.Errorf("path = %q, want /client/v4/zones/test-zone/dns_records", gotPath)
	}
}

func TestCreateRecord_APIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(cfResponse[dnsRecord]{
			Success: false,
			Errors:  []cfError{{Code: 1003, Message: "Invalid API key"}},
		})
	})
	_, err := c.CreateRecord(context.Background(), "test.com", "1.2.3.4")
	if err == nil {
		t.Error("CreateRecord should error on API failure")
	}
}

func TestDeleteRecord_Success(t *testing.T) {
	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		json.NewEncoder(w).Encode(cfResponse[any]{Success: true})
	})
	err := c.DeleteRecord(context.Background(), "rec-123")
	if err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if gotPath != "/client/v4/zones/test-zone/dns_records/rec-123" {
		t.Errorf("path = %q, want /client/v4/zones/test-zone/dns_records/rec-123", gotPath)
	}
}

func TestDeleteRecord_NotFoundIsSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	err := c.DeleteRecord(context.Background(), "rec-123")
	if err != nil {
		t.Errorf("DeleteRecord on 404 should be nil, got %v", err)
	}
}

func TestFindRecord_Success(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(cfResponse[[]dnsRecord]{
			Success: true,
			Result: []dnsRecord{{ID: "rec-456"}},
		})
	})
	id, err := c.FindRecord(context.Background(), "test.example.com")
	if err != nil {
		t.Fatalf("FindRecord: %v", err)
	}
	if id != "rec-456" {
		t.Errorf("id = %q, want rec-456", id)
	}
}

func TestFindRecord_NotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(cfResponse[[]dnsRecord]{
			Success: true,
			Result: []dnsRecord{},
		})
	})
	id, err := c.FindRecord(context.Background(), "missing.example.com")
	if err != nil {
		t.Fatalf("FindRecord: %v", err)
	}
	if id != "" {
		t.Errorf("id = %q, want empty", id)
	}
}

func TestEnsureRecord_CreatesWhenNotFound(t *testing.T) {
	callCount := 0
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		callCount++
		if req.Method == http.MethodGet {
			// FindRecord returns empty
			json.NewEncoder(w).Encode(cfResponse[[]dnsRecord]{Success: true, Result: []dnsRecord{}})
		} else {
			// CreateRecord
			json.NewEncoder(w).Encode(cfResponse[dnsRecord]{Success: true, Result: dnsRecord{ID: "new-rec"}})
		}
	})
	id, err := c.EnsureRecord(context.Background(), "test.com", "1.2.3.4")
	if err != nil {
		t.Fatalf("EnsureRecord: %v", err)
	}
	if id != "new-rec" {
		t.Errorf("id = %q, want new-rec", id)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (find + create)", callCount)
	}
}

func TestEnsureRecord_ReturnsExisting(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, req *http.Request) {
		// FindRecord returns existing
		json.NewEncoder(w).Encode(cfResponse[[]dnsRecord]{
			Success: true,
			Result: []dnsRecord{{ID: "existing-rec"}},
		})
	})
	id, err := c.EnsureRecord(context.Background(), "test.com", "1.2.3.4")
	if err != nil {
		t.Fatalf("EnsureRecord: %v", err)
	}
	if id != "existing-rec" {
		t.Errorf("id = %q, want existing-rec", id)
	}
}
