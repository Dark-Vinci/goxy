package main

const createUpstreamsSQL = `
	CREATE TABLE IF NOT EXISTS upstreams (
		addr TEXT PRIMARY KEY,
		role TEXT,
		healthy BOOLEAN,
		lag INTEGER
	)`

const createUsersSQL = `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password TEXT,
		role TEXT
	)`

const createSQLLog = `
    CREATE TABLE IF NOT EXISTS logs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        level TEXT,
        timestamp INTEGER,
        caller TEXT,
        message TEXT,
        fields TEXT
    );`

const createUpstreamHealth = `CREATE TABLE IF NOT EXISTS upstream_cron (
    id TEXT NOT NULL PRIMARY KEY,
    healthy INTEGER NOT NULL,        -- store bool as 0/1
    lag INTEGER,                     -- assuming lag is an int
    address TEXT NOT NULL,
    state_change INTEGER NOT NULL,   -- also 0/1
    nth INTEGER                     -- your p.nthCheck value
);`
