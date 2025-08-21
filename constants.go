package main

type QueryClass int

const (
	QueryUnknown QueryClass = iota
	QueryRead
	QueryWrite
)

type UpstreamRole int

//
//const (
//	RolePrimary UpstreamRole = iota
//	RoleReplica
//)

// UserRole defines user roles for RBAC
type UserRole string

const (
	UserRoleAdmin     UserRole = "admin"
	UserRoleReadWrite UserRole = "read_write"
	UserRoleReadOnly  UserRole = "read_only"
)

const TokenKey = "token"
