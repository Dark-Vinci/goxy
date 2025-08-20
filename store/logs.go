package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type LogsInterface interface {
	GetPaginatedLogs(ctx context.Context, requestID uuid.UUID, page, pageSize int, levelFilter string) (PaginatedResult[[]LogEntry], error)
	GetRequestIDLogs(ctx context.Context, requestID uuid.UUID, requestRequestID uuid.UUID) (PaginatedResult[[]LogEntry], error)
}

type LogStore struct {
	db  *gorm.DB
	log *zerolog.Logger
}

func NewLogStore(db *gorm.DB, log *zerolog.Logger) LogsInterface {
	return &LogStore{
		db:  db,
		log: log,
	}
}
func (l *LogStore) GetRequestIDLogs(ctx context.Context, requestID uuid.UUID, requestRequestID uuid.UUID) (PaginatedResult[[]LogEntry], error) {
	//TODO implement me
	panic("implement me")
}

var _ LogsInterface = (*LogStore)(nil)

func (l *LogStore) GetPaginatedLogs(ctx context.Context, requestID uuid.UUID, page, pageSize int, levelFilter string) (PaginatedResult[[]LogEntry], error) {
	log := l.log.With().
		Str(MethodStrHelper, "logs.GetPaginatedLogs").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get paginated logs")

	offset := (page - 1) * pageSize
	result := PaginatedResult[[]LogEntry]{
		Result:   []LogEntry{},
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

	if err := query.Find(&result.Result).Error; err != nil {
		log.Err(err).Msg("Failed to get paginated logs")
		return result, err
	}

	return result, nil
}
