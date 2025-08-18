package store

import (
	"gorm.io/gorm"
)

type Logs interface {
	GetPaginatedLogs(page, pageSize int, levelFilter string) (PaginatedLogsResult, error)
}

type LogStore struct {
	db *gorm.DB
}

var _ Logs = (*LogStore)(nil)

func (l *LogStore) GetPaginatedLogs(page, pageSize int, levelFilter string) (PaginatedLogsResult, error) {
	offset := (page - 1) * pageSize
	result := PaginatedLogsResult{
		Logs:     []LogEntry{},
		Page:     page,
		PageSize: pageSize,
	}

	// Count total logs
	var totalCount int64
	countQuery := l.db.Model(&LogEntry{})
	if levelFilter != "" {
		countQuery = countQuery.Where("level = ?", levelFilter)
	}

	if err := countQuery.Count(&totalCount).Error; err != nil {
		return result, err
	}
	result.TotalCount = totalCount

	// Query logs with pagination
	query := l.db.Model(&LogEntry{}).Order("timestamp DESC").Limit(pageSize).Offset(offset)
	if levelFilter != "" {
		query = query.Where("level = ?", levelFilter)
	}

	if err := query.Find(&result.Logs).Error; err != nil {
		return result, err
	}

	return result, nil
}
