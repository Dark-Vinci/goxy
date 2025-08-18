package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type UserInterface interface {
	Create(ctx context.Context, requestID uuid.UUID, payload User) (*User, error)
	Update(ctx context.Context, requestID uuid.UUID, payload User) error
	GetByID(ctx context.Context, requestID uuid.UUID, userID uuid.UUID) (*User, error)
	GetByUsername(ctx context.Context, requestID uuid.UUID, username string) (*User, error)
	Delete(ctx context.Context, requestID uuid.UUID, userID uuid.UUID, now time.Time) error
}

// Compile-time check
var _ UserInterface = (*UserStore)(nil)

type UserStore struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func (u UserStore) Create(ctx context.Context, requestID uuid.UUID, payload User) (*User, error) {
	log := u.logger.With().
		Str(MethodStrHelper, "user.Create").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to create user")

	if err := u.db.WithContext(ctx).Create(&payload).Error; err != nil {
		log.Err(err).Msg("Failed to get create user")
		return nil, err
	}

	return &payload, nil
}

func (u UserStore) Update(ctx context.Context, requestID uuid.UUID, payload User) error {
	log := u.logger.With().
		Str(MethodStrHelper, "user.Update").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to update user")

	if err := u.db.WithContext(ctx).
		Model(&User{}).
		Where("id = ?", requestID).
		Updates(payload).Error; err != nil {
		return err
	}

	return nil
}

func (u UserStore) GetByID(ctx context.Context, requestID uuid.UUID, userID uuid.UUID) (*User, error) {
	log := u.logger.With().
		Str(MethodStrHelper, "user.GetByID").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get a user by id")

	var user User

	if err := u.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error; err != nil {
		log.Err(err).Msg("Failed to get user by id")
		return nil, err
	}

	return &user, nil
}

func (u UserStore) GetByUsername(ctx context.Context, requestID uuid.UUID, username string) (*User, error) {
	log := u.logger.With().
		Str(MethodStrHelper, "user.GetByUsername").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msg("Got a request to get a user by username")

	var user User

	if err := u.db.WithContext(ctx).Where("email = ?", username).First(&user).Error; err != nil {
		log.Err(err).Msg("Failed to get user by username")
		return nil, err
	}

	return &user, nil
}

func (u UserStore) Delete(ctx context.Context, requestID uuid.UUID, userID uuid.UUID, now time.Time) error {
	log := u.logger.With().
		Str(MethodStrHelper, "channel.DeleteByID").
		Str(RequestID, requestID.String()).
		Logger()

	log.Info().Msgf("Got request to delete channel with ID %v", userID)

	if err := u.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).UpdateColumns(User{DeletedAt: &now}).Error; err != nil {
		log.Err(err).Msg("Failed to delete channel")

		return err
	}

	return nil
}
