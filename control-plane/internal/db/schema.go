package db

const schemaSQL = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS origins (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	wireguard_ip TEXT NOT NULL UNIQUE,
	wireguard_public_key TEXT,
	wireguard_private_key_encrypted TEXT,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS routes (
	id TEXT PRIMARY KEY,
	hostname TEXT NOT NULL UNIQUE,
	origin_id TEXT NOT NULL REFERENCES origins(id) ON DELETE CASCADE,
	target_port INTEGER NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS edge_nodes (
	id TEXT PRIMARY KEY,
	token_hash TEXT NOT NULL UNIQUE,
	name TEXT,
	wg_addr TEXT,
	wg_endpoint TEXT,
	wg_peer_pubkey TEXT,
	wg_allowed_ips TEXT,
	created_at DATETIME NOT NULL,
	last_seen DATETIME
);
`
