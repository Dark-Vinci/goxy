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
