package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/neo/kokoa-proxy/control-plane/internal/db"
	"github.com/neo/kokoa-proxy/control-plane/internal/generator"
	"github.com/neo/kokoa-proxy/control-plane/internal/web"
)

type ServerConfig struct {
	Store          *db.Store
	BootstrapToken string
	Logger         *log.Logger
}

type Server struct {
	store          *db.Store
	bootstrapToken string
	logger         *log.Logger
}

func NewServer(cfg ServerConfig) *Server {
	return &Server{
		store:          cfg.Store,
		bootstrapToken: cfg.BootstrapToken,
		logger:         cfg.Logger,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/origins", s.handleCreateOrigin)
	mux.HandleFunc("/api/v1/routes", s.handleCreateRoute)
	mux.HandleFunc("/api/v1/edge-nodes/register", s.handleRegisterEdgeNode)
	mux.HandleFunc("/api/v1/edge-nodes/me/config", s.handleEdgeConfig)
	mux.Handle("/", web.Handler())
	return s.logRequests(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateOrigin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name                         string `json:"name"`
		WireguardIP                  string `json:"wg_ip"`
		WireguardPublicKey           string `json:"wireguard_public_key"`
		WireguardPrivateKeyEncrypted string `json:"wireguard_private_key_encrypted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Name == "" || req.WireguardIP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and wg_ip are required"})
		return
	}
	origin, err := s.store.CreateOrigin(r.Context(), db.CreateOriginParams{
		Name:                         req.Name,
		WireguardIP:                  req.WireguardIP,
		WireguardPublicKey:           req.WireguardPublicKey,
		WireguardPrivateKeyEncrypted: req.WireguardPrivateKeyEncrypted,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, origin)
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Hostname   string `json:"hostname"`
		OriginID   string `json:"origin_id"`
		TargetPort int    `json:"target_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Hostname == "" || req.OriginID == "" || req.TargetPort == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hostname, origin_id and target_port are required"})
		return
	}
	route, err := s.store.CreateRoute(r.Context(), db.CreateRouteParams{
		Hostname:   req.Hostname,
		OriginID:   req.OriginID,
		TargetPort: req.TargetPort,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (s *Server) handleRegisterEdgeNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if s.bootstrapToken == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "bootstrap token is not configured"})
		return
	}
	if !s.validateBootstrapToken(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bootstrap token"})
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	issuedToken := uuid.NewString()
	node, err := s.store.RegisterEdgeNode(r.Context(), db.RegisterEdgeNodeParams{
		TokenPlain: issuedToken,
		Name:       req.Name,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"edge_node_id": node.ID,
		"token":        issuedToken,
	})
}

func (s *Server) handleEdgeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return
	}
	node, err := s.store.EdgeNodeByToken(r.Context(), token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	_ = s.store.TouchEdgeNode(r.Context(), node.ID, time.Now())

	routes, err := s.store.ListRoutes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load routes"})
		return
	}
	config := generator.BuildConfig(routes)
	writeJSON(w, http.StatusOK, map[string]any{
		"config_hash": config.ConfigHash,
		"hostnames":   config.Hostnames,
		"nginx_map":   config.Map,
		"routes":      config.Routes,
	})
}

func (s *Server) validateBootstrapToken(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") && strings.TrimPrefix(header, "Bearer ") == s.bootstrapToken {
		return true
	}
	if r.Header.Get("X-Bootstrap-Token") == s.bootstrapToken {
		return true
	}
	return false
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.logger != nil {
			s.logger.Printf("%s %s", r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(header string) (string, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
