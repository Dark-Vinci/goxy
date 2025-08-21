package main

import (
	"context"
	"fmt"
	"net"
	"thesis/store"
	"time"

	"github.com/google/uuid"
)

type Request struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Sql         []SQL
	CreatedAt   time.Time
	CompletedAt *time.Time
	conn        net.Conn
	connID      uint64
	ctx         context.Context
	requestID   uuid.UUID
	serverAddr  *string
}

type SQL struct {
	Sql         string
	CreatedAt   time.Time
	CompletedAt *time.Time
	IsRead      bool
}

func (p *Proxy) InsertRequest(request Request) error {
	log := p.logger.With().Str("request_id", request.requestID.String()).Logger()

	log.Info().Msgf("Inserting request: %v", request)
	dbReq := request.IntoDBRequest()

	if _, err := p.store.requestStore.Create(context.Background(), request.requestID, &dbReq); err != nil {
		log.Error().Err(err).Msgf("Failed to insert request into database: %v", err)
		return err
	}

	return nil
}

func (p *Proxy) InsertSQLS(request Request) error {
	log := p.logger.With().Str("request_id", uuid.NewString()).Logger()

	log.Info().Msgf("Inserting SQLS")

	for _, v := range request.IntoDBSQL() {
		if err := p.store.sqlStore.Create(context.Background(), v.RequestID, v); err != nil {
			log.Error().Err(err).Msgf("Failed to insert request into database: %v", err)
		}
	}

	return nil
}

func (r *Request) IntoDBRequest() store.Request {
	return store.Request{
		ID:          r.ID,
		UserID:      r.UserID,
		CreatedAt:   r.CreatedAt,
		CompletedAt: r.CompletedAt,
		ConnID:      r.connID,
		ServerAddr:  r.serverAddr,
	}
}

func (r *Request) IntoDBSQL() []store.SQL {
	result := make([]store.SQL, 0)

	for _, v := range r.Sql {
		result = append(result, store.SQL{
			ID:          uuid.New(),
			RequestID:   r.ID,
			Sql:         v.Sql,
			CreatedAt:   v.CreatedAt,
			CompletedAt: v.CompletedAt,
			IsRead:      v.IsRead,
		})
	}

	return result
}

func (r *Request) String() string {
	return fmt.Sprintf("ID: %v, UserID: %v, SQL: %v, CreatedAt: %v, CompletedAt: %v", r.ID, r.Sql, r.Sql, r.CreatedAt, r.CompletedAt)
}
