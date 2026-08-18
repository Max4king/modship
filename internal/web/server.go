// Package web implements the modship HTTP UI using chi router + htmx templates.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ryan3311/modship/internal/deploy"
	"github.com/ryan3311/modship/internal/model"
	"github.com/ryan3311/modship/internal/provider"
	"github.com/ryan3311/modship/internal/store"
)

// Server is the HTTP UI server.
type Server struct {
	store    *store.Store
	registry *provider.Registry
	deploy   *deploy.Manager
	router   chi.Router
}

// New creates the web server and registers all routes.
func New(s *store.Store, reg *provider.Registry, dm *deploy.Manager) *Server {
	srv := &Server{
		store:    s,
		registry: reg,
		deploy:   dm,
		router:   chi.NewRouter(),
	}
	srv.routes()
	return srv
}

// Handler returns the http.Handler for use with http.Server.
func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() {
	r := s.router

	// Static assets (embedded templates served at /).
	r.Get("/", s.handleIndex)
	r.Get("/servers", s.handleListServers)
	r.Get("/servers/{id}", s.handleGetServer)

	// Search + versions (provider API).
	r.Get("/api/search", s.handleSearch)
	r.Get("/api/versions", s.handleVersions)

	// Server lifecycle.
	r.Post("/api/servers", s.handleDeploy)
	r.Post("/api/servers/{id}/start", s.handleStart)
	r.Post("/api/servers/{id}/stop", s.handleStop)
	r.Delete("/api/servers/{id}", s.handleDelete)
}

// handleIndex renders the main page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	servers, _ := s.store.ListServers(r.Context())
	renderPage(w, "index", map[string]any{
		"Servers": servers,
	})
}

// handleListServers returns the server list as an HTML fragment (htmx).
func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.store.ListServers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderFragment(w, "server_list", map[string]any{"Servers": servers})
}

// handleGetServer returns a single server detail as HTML fragment.
func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	srv, err := s.store.GetServer(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	renderFragment(w, "server_detail", map[string]any{"Server": srv})
}

// handleSearch proxies a search query to the appropriate provider.
// It accepts optional page (0-indexed) and pageSize query params, with
// defaults of page=0 and pageSize=10.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	providerName := r.URL.Query().Get("provider")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query param 'q' is required"})
		return
	}
	page, pageSize := 0, 10
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'page' param: must be a non-negative integer"})
			return
		}
		page = n
	}
	if v := r.URL.Query().Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'pageSize' param: must be a positive integer"})
			return
		}
		pageSize = n
	}
	p := s.registry.Get(model.Provider(providerName))
	if p == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown provider: " + providerName})
		return
	}
	results, err := p.Search(r.Context(), q, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// handleVersions returns versions for a modpack.
func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	providerName := r.URL.Query().Get("provider")
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query param 'slug' is required"})
		return
	}
	p := s.registry.Get(model.Provider(providerName))
	if p == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown provider: " + providerName})
		return
	}
	versions, err := p.GetVersions(r.Context(), slug)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// handleDeploy creates a new server deployment.
func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var req deploy.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	srv, err := s.deploy.Deploy(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, srv)
}

// handleStart starts a stopped server.
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.deploy.Start(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// handleStop stops a running server.
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.deploy.Stop(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// handleDelete removes a server entirely.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.deploy.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// renderPage renders a full HTML page. Templates are embedded.
func renderPage(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name+".html", data); err != nil {
		fmt.Fprintf(w, "<p>template error: %s</p>", err)
	}
}

// renderFragment renders a partial HTML fragment.
func renderFragment(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name+".html", data); err != nil {
		fmt.Fprintf(w, "<p>template error: %s</p>", err)
	}
}

// init ensures the strings import is used in case of future helpers.
var _ = strings.TrimSpace

// Ensure context is used.
var _ context.Context = nil
