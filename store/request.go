package store

import (
	"context"
	"github.com/google/uuid"
)

type RequestInterface interface {
	Create(ctx context.Context, requestID uuid.UUID, payload uuid.UUID) (*uuid.UUID, error)
	GetPaginatedRequest(ctx context.Context, requestID uuid.UUID, page int, limit int) ([]Request, error)
}
