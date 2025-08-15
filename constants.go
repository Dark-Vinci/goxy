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

// UserRole defines user roles for RBAC
type UserRole string

const (
	UserRoleAdmin     UserRole = "admin"
	UserRoleReadWrite UserRole = "read_write"
	UserRoleReadOnly  UserRole = "read_only"
)

type ConnPool *net.Conn

var writePrefixes = []string{
	"INSERT", "UPDATE", "DELETE", "MERGE", "CREATE", "ALTER", "DROP", "TRUNCATE",
	"GRANT", "REVOKE", "VACUUM", "ANALYZE", "REINDEX", "REFRESH", "CALL",
	"COPY ", // COPY table FROM … writes; COPY … TO is read-ish but keep simple
	"LOCK", "CLUSTER", "DISCARD", "SECURITY LABEL",
}
