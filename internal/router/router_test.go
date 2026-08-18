package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestRouter(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return New(ts.URL)
}

func TestAddRoute_Success(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]string
	r := newTestRouter(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		json.NewDecoder(req.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	})
	err := r.AddRoute(context.Background(), "test.example.com", "test:25565")
	if err != nil {
		t.Fatalf("AddRoute: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/routes" {
		t.Errorf("path = %q, want /routes", gotPath)
	}
	if body["host"] != "test.example.com" || body["backend"] != "test:25565" {
		t.Errorf("body = %v, want host=test.example.com backend=test:25565", body)
	}
}

func TestAddRoute_Error(t *testing.T) {
	r := newTestRouter(t, func(w http.ResponseWriter, req *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	})
	err := r.AddRoute(context.Background(), "test.com", "test:25565")
	if err == nil {
		t.Error("AddRoute should error on 500")
	}
}

func TestRemoveRoute_Success(t *testing.T) {
	var gotMethod, gotPath string
	r := newTestRouter(t, func(w http.ResponseWriter, req *http.Request) {
		gotMethod = req.Method
		gotPath = req.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	err := r.RemoveRoute(context.Background(), "test.example.com")
	if err != nil {
		t.Fatalf("RemoveRoute: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/routes/test.example.com" {
		t.Errorf("path = %q, want /routes/test.example.com", gotPath)
	}
}

func TestRemoveRoute_NotFoundIsSuccess(t *testing.T) {
	r := newTestRouter(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	err := r.RemoveRoute(context.Background(), "test.com")
	if err != nil {
		t.Errorf("RemoveRoute on 404 should be nil, got %v", err)
	}
}

func TestRemoveRoute_ServerError(t *testing.T) {
	r := newTestRouter(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := r.RemoveRoute(context.Background(), "test.com")
	if err == nil {
		t.Error("RemoveRoute should error on 500")
	}
}

func TestListRoutes_Success(t *testing.T) {
	r := newTestRouter(t, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"a.example.com": "a:25565",
			"b.example.com": "b:25565",
		})
	})
	routes, err := r.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("ListRoutes: %v", err)
	}
	if len(routes) != 2 {
		t.Errorf("len = %d, want 2", len(routes))
	}
	if routes["a.example.com"] != "a:25565" {
		t.Errorf("routes[a] = %q, want a:25565", routes["a.example.com"])
	}
}
