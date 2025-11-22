package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neo/kokoa-proxy/control-plane/internal/db"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewServer(ServerConfig{Store: store})
}

func TestCreateOriginValidation(t *testing.T) {
	srv := newTestServer(t)
	body := `{"name":"o1","wg_ip":"not-an-ip"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/origins", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateRouteValidationAndSuccess(t *testing.T) {
	srv := newTestServer(t)

	origin, err := srv.store.CreateOrigin(context.Background(), db.CreateOriginParams{
		Name:        "o1",
		WireguardIP: "10.0.0.2",
	})
	if err != nil {
		t.Fatalf("create origin: %v", err)
	}

	// invalid port
	badBody := `{"hostname":"app.example.com","origin_id":"` + origin.ID + `","target_port":70000}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/routes", strings.NewReader(badBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid port, got %d", rec.Code)
	}

	// valid route
	good := map[string]any{
		"hostname":    "app.example.com",
		"origin_id":   origin.ID,
		"target_port": 8080,
	}
	buf, _ := json.Marshal(good)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/routes", bytes.NewReader(buf))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec2.Code)
	}
}
