package main

const createLogEntryTable = `
CREATE TABLE IF NOT EXISTS log_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    level TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    caller TEXT,
    message TEXT,
    fields TEXT  -- JSON string, since SQLite doesn’t have a native map type
);`

const createHealthChecks = `
CREATE TABLE IF NOT EXISTS health_checks (
	id TEXT PRIMARY KEY,
	addr TEXT NOT NULL,
	healthy INTEGER NOT NULL,
	lag INTEGER NOT NULL,
	state_change INTEGER NOT NULL,
	created_at DATETIME NOT NULL
);`

const createUserTable = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	is_admin INTEGER NOT NULL,
	role TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME
);`

const createRequestTable = `
CREATE TABLE IF NOT EXISTS requests (
	id TEXT PRIMARY KEY,
	user_id TEXT,
	sql TEXT,
	created_at DATETIME NOT NULL,
	completed_at DATETIME,
	conn_id INTEGER,
	server_addr TEXT
);`

const createSQLTable = `
CREATE TABLE IF NOT EXISTS sqls (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    sql TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    completed_at DATETIME,
    is_read BOOLEAN NOT NULL DEFAULT 0
);`
