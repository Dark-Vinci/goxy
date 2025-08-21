package store

import (
	"context"
	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type SQLInterface interface {
	Create(ctx context.Context, requestID uuid.UUID, payload SQL) error
	GetRequestSQL(ctx context.Context, requestID uuid.UUID, requestRequestID uuid.UUID) ([]SQL, error)
	GetPaginatedSQL(ctx context.Context, requestID uuid.UUID, page, pageSize int, isRead *bool) (PaginatedResult[[]SQL], error)
}

type SQLStore struct {
	db  *gorm.DB
	log *zerolog.Logger
}

var _ SQLInterface = (*SQLStore)(nil)

func NewSQLStore(db *gorm.DB, log *zerolog.Logger) SQLInterface {
	return &SQLStore{
		db:  db,
		log: log,
	}
}

func (s *SQLStore) GetPaginatedSQL(ctx context.Context, requestID uuid.UUID, page, pageSize int, isRead *bool) (PaginatedResult[[]SQL], error) {
	log := s.log.With().
		Str(MethodStrHelper, "sql.GetPaginatedSQL").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get paginated SQL")

	offset := (page - 1) * pageSize

	result := PaginatedResult[[]SQL]{
		Result:   []SQL{},
		Page:     page,
		PageSize: pageSize,
	}

	q := s.db.WithContext(ctx).
		Model(&SQL{}).
		Offset(offset).
		Limit(pageSize)

	if isRead != nil {
		q.Where("is_read = ?", *isRead)
	}

	if err := q.Find(&result.Result).Error; err != nil {
		log.Err(err).Msg("Failed to get paginated SQL")
		return result, err
	}

	return result, nil
}

func (s *SQLStore) Create(ctx context.Context, requestID uuid.UUID, payload SQL) error {
	log := s.log.With().
		Str(MethodStrHelper, "sql.Create").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to create SQL")

	if err := s.db.WithContext(ctx).Create(&payload).Error; err != nil {
		log.Err(err).Msg("Failed to get create SQL")
		return err
	}

	return nil
}

func (s *SQLStore) GetRequestSQL(ctx context.Context, requestID uuid.UUID, requestRequestID uuid.UUID) ([]SQL, error) {
	log := s.log.With().
		Str(MethodStrHelper, "sql.GetRequestSQL").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get SQL")

	var sqls []SQL

	if err := s.db.WithContext(ctx).Where("request_id = ?", requestRequestID).Find(&sqls).Error; err != nil {
		log.Err(err).Msg("Failed to get SQL")
		return nil, err
	}

	return sqls, nil
}
