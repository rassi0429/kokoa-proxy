package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite" // SQLite driver
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	dbh, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	dbh.SetMaxOpenConns(1)
	dbh.SetMaxIdleConns(1)
	dbh.SetConnMaxLifetime(0)

	return &Store{db: dbh}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

type Origin struct {
	ID                           string
	Name                         string
	WireguardIP                  string
	WireguardPublicKey           string
	WireguardPrivateKeyEncrypted string
	CreatedAt                    time.Time
}

type CreateOriginParams struct {
	Name                         string
	WireguardIP                  string
	WireguardPublicKey           string
	WireguardPrivateKeyEncrypted string
}

func (s *Store) CreateOrigin(ctx context.Context, params CreateOriginParams) (Origin, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO origins (id, name, wireguard_ip, wireguard_public_key, wireguard_private_key_encrypted, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, params.Name, params.WireguardIP, params.WireguardPublicKey, params.WireguardPrivateKeyEncrypted, now)
	if err != nil {
		return Origin{}, fmt.Errorf("insert origin: %w", err)
	}
	return Origin{
		ID:                           id,
		Name:                         params.Name,
		WireguardIP:                  params.WireguardIP,
		WireguardPublicKey:           params.WireguardPublicKey,
		WireguardPrivateKeyEncrypted: params.WireguardPrivateKeyEncrypted,
		CreatedAt:                    now,
	}, nil
}

type Route struct {
	ID         string
	Hostname   string
	OriginID   string
	TargetPort int
	CreatedAt  time.Time
}

type CreateRouteParams struct {
	Hostname   string
	OriginID   string
	TargetPort int
}

func (s *Store) CreateRoute(ctx context.Context, params CreateRouteParams) (Route, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO routes (id, hostname, origin_id, target_port, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, params.Hostname, params.OriginID, params.TargetPort, now)
	if err != nil {
		return Route{}, fmt.Errorf("insert route: %w", err)
	}
	return Route{
		ID:         id,
		Hostname:   params.Hostname,
		OriginID:   params.OriginID,
		TargetPort: params.TargetPort,
		CreatedAt:  now,
	}, nil
}

type EdgeNode struct {
	ID        string
	Name      string
	TokenHash string
	CreatedAt time.Time
	LastSeen  sql.NullTime
}

type RegisterEdgeNodeParams struct {
	TokenPlain string
	Name       string
}

func (s *Store) RegisterEdgeNode(ctx context.Context, params RegisterEdgeNodeParams) (EdgeNode, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	tokenHash := sha256.Sum256([]byte(params.TokenPlain))
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO edge_nodes (id, name, token_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, id, params.Name, fmt.Sprintf("%x", tokenHash[:]), now)
	if err != nil {
		return EdgeNode{}, fmt.Errorf("insert edge node: %w", err)
	}
	return EdgeNode{
		ID:        id,
		Name:      params.Name,
		TokenHash: fmt.Sprintf("%x", tokenHash[:]),
		CreatedAt: now,
	}, nil
}

func (s *Store) EdgeNodeByToken(ctx context.Context, token string) (EdgeNode, error) {
	tokenHash := sha256.Sum256([]byte(token))
	var node EdgeNode
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, token_hash, created_at, last_seen
		FROM edge_nodes
		WHERE token_hash = ?
	`, fmt.Sprintf("%x", tokenHash[:])).Scan(&node.ID, &node.Name, &node.TokenHash, &node.CreatedAt, &node.LastSeen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EdgeNode{}, err
		}
		return EdgeNode{}, fmt.Errorf("select edge node: %w", err)
	}
	return node, nil
}

func (s *Store) TouchEdgeNode(ctx context.Context, id string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE edge_nodes SET last_seen = ? WHERE id = ?
	`, at.UTC(), id)
	if err != nil {
		return fmt.Errorf("update last_seen: %w", err)
	}
	return nil
}

type RouteWithOrigin struct {
	Hostname    string
	TargetPort  int
	OriginID    string
	OriginName  string
	WireguardIP string
}

func (s *Store) ListRoutes(ctx context.Context) ([]RouteWithOrigin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.hostname, r.target_port, r.origin_id, o.name, o.wireguard_ip
		FROM routes r
		INNER JOIN origins o ON r.origin_id = o.id
	`)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var out []RouteWithOrigin
	for rows.Next() {
		var r RouteWithOrigin
		if err := rows.Scan(&r.Hostname, &r.TargetPort, &r.OriginID, &r.OriginName, &r.WireguardIP); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListEdgeNodes(ctx context.Context) ([]EdgeNode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, token_hash, created_at, last_seen
		FROM edge_nodes
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list edge nodes: %w", err)
	}
	defer rows.Close()

	var out []EdgeNode
	for rows.Next() {
		var n EdgeNode
		if err := rows.Scan(&n.ID, &n.Name, &n.TokenHash, &n.CreatedAt, &n.LastSeen); err != nil {
			return nil, fmt.Errorf("scan edge node: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) ListOrigins(ctx context.Context) ([]Origin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, wireguard_ip, wireguard_public_key, wireguard_private_key_encrypted, created_at
		FROM origins
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list origins: %w", err)
	}
	defer rows.Close()

	var out []Origin
	for rows.Next() {
		var o Origin
		if err := rows.Scan(&o.ID, &o.Name, &o.WireguardIP, &o.WireguardPublicKey, &o.WireguardPrivateKeyEncrypted, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan origin: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
