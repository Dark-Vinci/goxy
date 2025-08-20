package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type RequestInterface interface {
	Create(ctx context.Context, requestID uuid.UUID, payload *Request) (*uuid.UUID, error)
	GetPaginatedRequest(ctx context.Context, requestID uuid.UUID, page int, pageSize int) (PaginatedResult[[]Request], error)
	GetByRequestID(ctx context.Context, requestID uuid.UUID, requestRequestID uuid.UUID) (*Request, error)
}

var _ RequestInterface = (*RequestStore)(nil)

type RequestStore struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func NewRequestStore(logger *zerolog.Logger, db *gorm.DB) RequestInterface {
	return &RequestStore{
		logger: logger,
		db:     db,
	}
}

func (r RequestStore) GetByRequestID(ctx context.Context, requestID uuid.UUID, requestRequestID uuid.UUID) (*Request, error) {
	log := r.logger.With().
		Str(MethodStrHelper, "request.GetByRequestID").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get a request by id")

	var request Request

	if err := r.db.WithContext(ctx).Where("request_id = ?", requestRequestID).First(&request).Error; err != nil {
		log.Err(err).Msg("Failed to get request by id")
		return nil, err
	}

	return &request, nil
}

func (r RequestStore) Create(ctx context.Context, requestID uuid.UUID, payload *Request) (*uuid.UUID, error) {
	log := r.logger.With().
		Str(MethodStrHelper, "request.Create").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to create request")

	if err := r.db.WithContext(ctx).Create(&payload).Error; err != nil {
		log.Err(err).Msg("Failed to get create request")
		return nil, err
	}

	return &payload.ID, nil
}

func (r RequestStore) GetPaginatedRequest(ctx context.Context, requestID uuid.UUID, page int, pageSize int) (PaginatedResult[[]Request], error) {
	log := r.logger.With().
		Str(MethodStrHelper, "request.GetPaginatedRequest").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get paginated request")

	offset := (page - 1) * pageSize
	result := PaginatedResult[[]Request]{
		Result:   []Request{},
		Page:     page,
		PageSize: pageSize,
	}

	if err := r.db.WithContext(ctx).
		Model(&Request{}).
		Offset(offset).
		Limit(pageSize).
		Find(&result.Result).Error; err != nil {

		log.Err(err).Msg("Failed to get paginated request")
		return result, err
	}

	return result, nil
}
