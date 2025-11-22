package api

import (
	"encoding/json"
	"log"
	"mime"
	"net/http"
	"net/netip"
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
	rateLimiter    *rateLimiter
}

func NewServer(cfg ServerConfig) *Server {
	return &Server{
		store:          cfg.Store,
		bootstrapToken: cfg.BootstrapToken,
		logger:         cfg.Logger,
		rateLimiter:    newRateLimiter(60, time.Minute),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/v1/origins", s.handleCreateOrigin)
	mux.HandleFunc("/api/v1/routes", s.handleCreateRoute)
	mux.HandleFunc("/api/v1/origins/list", s.handleListOrigins)
	mux.HandleFunc("/api/v1/routes/list", s.handleListRoutes)
	mux.HandleFunc("/api/v1/edge-nodes/register", s.handleRegisterEdgeNode)
	mux.HandleFunc("/api/v1/edge-nodes/me/config", s.handleEdgeConfig)
	mux.Handle("/", web.Handler())
	return s.logRequests(s.applyRateLimit(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateOrigin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mt, _, _ := mime.ParseMediaType(ct)
		if mt != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "content-type must be application/json")
			return
		}
	}
	var req struct {
		Name                         string `json:"name"`
		WireguardIP                  string `json:"wg_ip"`
		WireguardPublicKey           string `json:"wireguard_public_key"`
		WireguardPrivateKeyEncrypted string `json:"wireguard_private_key_encrypted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateOrigin(req.Name, req.WireguardIP); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	origin, err := s.store.CreateOrigin(r.Context(), db.CreateOriginParams{
		Name:                         req.Name,
		WireguardIP:                  req.WireguardIP,
		WireguardPublicKey:           req.WireguardPublicKey,
		WireguardPrivateKeyEncrypted: req.WireguardPrivateKeyEncrypted,
	})
	if err != nil {
		status := http.StatusBadRequest
		if isConstraintError(err) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, origin)
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Hostname   string `json:"hostname"`
		OriginID   string `json:"origin_id"`
		TargetPort int    `json:"target_port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateRoute(req.Hostname, req.TargetPort, req.OriginID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	route, err := s.store.CreateRoute(r.Context(), db.CreateRouteParams{
		Hostname:   req.Hostname,
		OriginID:   req.OriginID,
		TargetPort: req.TargetPort,
	})
	if err != nil {
		status := http.StatusBadRequest
		if isConstraintError(err) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (s *Server) handleRegisterEdgeNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.bootstrapToken == "" {
		writeError(w, http.StatusServiceUnavailable, "bootstrap token is not configured")
		return
	}
	if !s.validateBootstrapToken(r) {
		writeError(w, http.StatusUnauthorized, "invalid bootstrap token")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	issuedToken := uuid.NewString()
	node, err := s.store.RegisterEdgeNode(r.Context(), db.RegisterEdgeNodeParams{
		TokenPlain: issuedToken,
		Name:       req.Name,
	})
	if err != nil {
		status := http.StatusBadRequest
		if isConstraintError(err) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"edge_node_id": node.ID,
		"token":        issuedToken,
	})
}

func (s *Server) handleEdgeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing bearer token")
		return
	}
	node, err := s.store.EdgeNodeByToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	_ = s.store.TouchEdgeNode(r.Context(), node.ID, time.Now())

	routes, err := s.store.ListRoutes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load routes")
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

func (s *Server) handleListOrigins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	list, err := s.store.ListOrigins(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list origins")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	list, err := s.store.ListRoutes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list routes")
		return
	}
	writeJSON(w, http.StatusOK, list)
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
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		if s.logger != nil {
			s.logger.Printf("method=%s path=%s status=%d dur=%s", r.Method, r.URL.Path, rec.status, time.Since(start))
		}
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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func validateOrigin(name, wgIP string) error {
	if strings.TrimSpace(name) == "" {
		return errf("name is required")
	}
	if wgIP == "" {
		return errf("wg_ip is required")
	}
	if _, err := netip.ParseAddr(wgIP); err != nil {
		return errf("wg_ip must be a valid IP address")
	}
	return nil
}

func validateRoute(hostname string, port int, originID string) error {
	if strings.TrimSpace(hostname) == "" || originID == "" {
		return errf("hostname and origin_id are required")
	}
	if !validHostname(hostname) {
		return errf("hostname is invalid")
	}
	if port < 1 || port > 65535 {
		return errf("target_port must be between 1 and 65535")
	}
	return nil
}

func validHostname(h string) bool {
	if strings.HasSuffix(h, ".") {
		h = strings.TrimSuffix(h, ".")
	}
	parts := strings.Split(h, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 || len(p) > 63 {
			return false
		}
		for i := 0; i < len(p); i++ {
			c := p[i]
			if !(c == '-' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
				return false
			}
		}
		if p[0] == '-' || p[len(p)-1] == '-' {
			return false
		}
	}
	return true
}

func errf(msg string) error {
	return &apiError{msg: msg}
}

type apiError struct {
	msg string
}

func (e *apiError) Error() string { return e.msg }

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "constraint failed")
}

func (s *Server) applyRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.rateLimiter != nil && !s.rateLimiter.allow(r.RemoteAddr) {
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
