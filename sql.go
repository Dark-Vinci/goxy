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
