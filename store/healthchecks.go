package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// change name from upstream_cron to healthchecks

type HealthCheckInterface interface {
	GetPaginatedHealthChecks(ctx context.Context, requestID uuid.UUID, page, pageSize int) (PaginatedResult[[]HealthCheck], error)
	GetFailedHealthChecks(ctx context.Context, requestID uuid.UUID, page, pageSize int) (PaginatedResult[[]HealthCheck], error)
}

type HealthCheckStore struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func NewHealthCheckStore(logger *zerolog.Logger, db *gorm.DB) HealthCheckInterface {
	return &HealthCheckStore{
		logger: logger,
		db:     db,
	}
}

func (h *HealthCheckStore) GetFailedHealthChecks(ctx context.Context, requestID uuid.UUID, page, pageSize int) (PaginatedResult[[]HealthCheck], error) {
	log := h.logger.With().
		Str(MethodStrHelper, "healthcheck.GetFailedHealthChecks").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get failed health checks")

	offset := (page - 1) * pageSize
	result := PaginatedResult[[]HealthCheck]{
		Result:   []HealthCheck{},
		Page:     page,
		PageSize: pageSize,
	}

	if err := h.db.WithContext(ctx).
		Model(&HealthCheck{}).
		Offset(offset).
		Limit(pageSize).
		Where("is_healthy = 0").
		Find(&result.Result).Error; err != nil {
		log.Err(err).Msg("Failed to get paginated failed health checks")
		return result, err
	}

	return result, nil
}

func (h *HealthCheckStore) GetPaginatedHealthChecks(ctx context.Context, requestID uuid.UUID, page, pageSize int) (PaginatedResult[[]HealthCheck], error) {
	log := h.logger.With().
		Str(MethodStrHelper, "healthcheck.GetPaginatedHealthChecks").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get health checks")

	offset := (page - 1) * pageSize
	result := PaginatedResult[[]HealthCheck]{
		Result:   []HealthCheck{},
		Page:     page,
		PageSize: pageSize,
	}

	if err := h.db.WithContext(ctx).
		Model(&HealthCheck{}).
		Offset(offset).
		Limit(pageSize).
		Find(&result.Result).Error; err != nil {
		log.Err(err).Msg("Failed to get paginated failed health checks")
		return result, err
	}

	return result, nil
}
