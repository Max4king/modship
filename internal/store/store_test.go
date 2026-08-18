package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ryan3311/modship/internal/model"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleServer() *model.Server {
	return &model.Server{
		ID:          uuid.NewString(),
		Name:        "test-server",
		Provider:    model.ProviderCurseForge,
		ModpackSlug: "test-pack",
		ModpackName: "Test Pack",
		FileID:      "12345",
		GameVersion: "1.20.1",
		JavaImage:   "java17",
		Memory:      "10G",
		Domain:      "test-server.example.com",
		Whitelist:   []string{"Player1", "Player2"},
		State:       model.StateCreating,
	}
}

func TestOpen_Migrations(t *testing.T) {
	s := newTestStore(t)
	// Table should exist — verify by inserting a row.
	if err := s.CreateServer(context.Background(), sampleServer()); err != nil {
		t.Fatalf("CreateServer after migration: %v", err)
	}
}

func TestCreateAndGetServer(t *testing.T) {
	s := newTestStore(t)
	srv := sampleServer()

	if err := s.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	got, err := s.GetServer(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if got.Name != srv.Name {
		t.Errorf("Name = %q, want %q", got.Name, srv.Name)
	}
	if got.Provider != srv.Provider {
		t.Errorf("Provider = %q, want %q", got.Provider, srv.Provider)
	}
	if got.ModpackSlug != srv.ModpackSlug {
		t.Errorf("ModpackSlug = %q, want %q", got.ModpackSlug, srv.ModpackSlug)
	}
	if got.FileID != srv.FileID {
		t.Errorf("FileID = %q, want %q", got.FileID, srv.FileID)
	}
	if got.Domain != srv.Domain {
		t.Errorf("Domain = %q, want %q", got.Domain, srv.Domain)
	}
	if got.State != srv.State {
		t.Errorf("State = %q, want %q", got.State, srv.State)
	}
	if len(got.Whitelist) != 2 || got.Whitelist[0] != "Player1" {
		t.Errorf("Whitelist = %v, want [Player1 Player2]", got.Whitelist)
	}
}

func TestCreateServer_DuplicateName(t *testing.T) {
	s := newTestStore(t)
	srv := sampleServer()
	if err := s.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer first: %v", err)
	}
	srv2 := sampleServer()
	srv2.ID = uuid.NewString() // different ID, same name
	if err := s.CreateServer(context.Background(), srv2); err == nil {
		t.Error("CreateServer with duplicate name should error")
	}
}

func TestGetServerByName(t *testing.T) {
	s := newTestStore(t)
	srv := sampleServer()
	if err := s.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	got, err := s.GetServerByName(context.Background(), srv.Name)
	if err != nil {
		t.Fatalf("GetServerByName: %v", err)
	}
	if got.ID != srv.ID {
		t.Errorf("ID = %q, want %q", got.ID, srv.ID)
	}
}

func TestListServers(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 3; i++ {
		srv := sampleServer()
		srv.ID = uuid.NewString()
		srv.Name = "server-" + string(rune('a'+i))
		if err := s.CreateServer(context.Background(), srv); err != nil {
			t.Fatalf("CreateServer %d: %v", i, err)
		}
	}
	list, err := s.ListServers(context.Background())
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
}

func TestUpdateServer(t *testing.T) {
	s := newTestStore(t)
	srv := sampleServer()
	if err := s.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	srv.State = model.StateRunning
	srv.ContainerID = "abc123"
	srv.CloudflareRecordID = "cf-record-id"

	if err := s.UpdateServer(context.Background(), srv); err != nil {
		t.Fatalf("UpdateServer: %v", err)
	}
	got, err := s.GetServer(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if got.State != model.StateRunning {
		t.Errorf("State = %q, want %q", got.State, model.StateRunning)
	}
	if got.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", got.ContainerID, "abc123")
	}
	if got.CloudflareRecordID != "cf-record-id" {
		t.Errorf("CloudflareRecordID = %q, want %q", got.CloudflareRecordID, "cf-record-id")
	}
}

func TestDeleteServer(t *testing.T) {
	s := newTestStore(t)
	srv := sampleServer()
	if err := s.CreateServer(context.Background(), srv); err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	if err := s.DeleteServer(context.Background(), srv.ID); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}
	if _, err := s.GetServer(context.Background(), srv.ID); err == nil {
		t.Error("GetServer after delete should error")
	}
}
