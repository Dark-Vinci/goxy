package main

import "net"

type QueryClass int

const (
	QueryUnknown QueryClass = iota
	QueryRead
	QueryWrite
)

type UpstreamRole int

const (
	RolePrimary UpstreamRole = iota
	RoleReplica
)

type ConnPool *net.Conn

var writePrefixes = []string{
	"INSERT", "UPDATE", "DELETE", "MERGE", "CREATE", "ALTER", "DROP", "TRUNCATE",
	"GRANT", "REVOKE", "VACUUM", "ANALYZE", "REINDEX", "REFRESH", "CALL",
	"COPY ", // COPY table FROM … writes; COPY … TO is read-ish but keep simple
	"LOCK", "CLUSTER", "DISCARD", "SECURITY LABEL",
}
