package store

import (
	"time"

	"github.com/google/uuid"
)

type HealthCheck struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()" json:"id"`
	Addr      string    `gorm:"not null" json:"addr"`
	Healthy   int       `gorm:"not null" json:"healthy"`
	Lag       int       `gorm:"not null" json:"lag"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

type User struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"not null" json:"password"`
	IsAdmin   bool      `gorm:"not null" json:"role"` // admin,
	Role      string
	CreatedAt time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null" json:"updated_at"`
	DeletedAt *time.Time `gorm:"not null" json:"deleted_at"`
}

type Request struct {
	ID           uuid.UUID  `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()" json:"id"`
	UserID       *uuid.UUID `gorm:"not null" json:"user_id"`
	Sql          string     `gorm:"not null" json:"sql"`
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
	CompletedAt  time.Time  `gorm:"not null" json:"completed_at"`
	Duration     int        `gorm:"not null" json:"duration"`
	ConnectionID uuid.UUID  `gorm:"not null" json:"connection_id"`
}

type LogEntry struct {
	ID        int64                  `json:"id"`
	Level     string                 `json:"level"`
	Timestamp int64                  `json:"timestamp"`
	Caller    string                 `json:"caller"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
}

// PaginatedResult holds the paginated query results
type PaginatedResult[T any] struct {
	Result     T     `json:"result"`
	TotalCount int64 `json:"total_count"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
}
