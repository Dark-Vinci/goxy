package store

import (
	"github.com/google/uuid"
	"time"
)

type User struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"not null" json:"password"`
	IsAdmin   string    `gorm:"not null" json:"role"` // admin,
	Role      string
	CreatedAt time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt time.Time  `gorm:"not null" json:"updated_at"`
	DeletedAt *time.Time `gorm:"not null" json:"deleted_at"`
}

type Request struct {
	ID           uuid.UUID `gorm:"primaryKey;type:uuid;default:uuid_generate_v4()" json:"id"`
	UserID       uuid.UUID `gorm:"not null" json:"user_id"`
	Sql          string    `gorm:"not null" json:"sql"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
	CompletedAt  time.Time `gorm:"not null" json:"completed_at"`
	Duration     int       `gorm:"not null" json:"duration"`
	ConnectionID uuid.UUID `gorm:"not null" json:"connection_id"`
}

type LogEntry struct {
	ID        int64                  `json:"id"`
	Level     string                 `json:"level"`
	Timestamp int64                  `json:"timestamp"`
	Caller    string                 `json:"caller"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
}

// PaginatedLogsResult holds the paginated query results
type PaginatedLogsResult struct {
	Logs       []LogEntry `json:"logs"`
	TotalCount int64      `json:"total_count"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}
