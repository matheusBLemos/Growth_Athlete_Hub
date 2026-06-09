package usecase

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type RegisterUserInput struct {
	Name      string
	Email     string
	Password  string
	BirthDate time.Time
}

type RegisterUserOutput struct {
	ID string
}

type RegisterUser struct {
	userRepo  port.UserRepository
	hasher    port.PasswordHasher
	publisher port.EventPublisher
}

func NewRegisterUser(repo port.UserRepository, hasher port.PasswordHasher, pub port.EventPublisher) *RegisterUser {
	return &RegisterUser{
		userRepo:  repo,
		hasher:    hasher,
		publisher: pub,
	}
}

func (uc *RegisterUser) Execute(ctx context.Context, input RegisterUserInput) (*RegisterUserOutput, error) {
	// Valida o e-mail criando primeiro a entidade base (sem hash ainda).
	base, err := entity.NewUser(input.Name, input.Email, input.BirthDate)
	if err != nil {
		return nil, err
	}

	existing, err := uc.userRepo.FindByEmail(ctx, base.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := uc.hasher.Hash(input.Password)
	if err != nil {
		return nil, err
	}

	user, err := entity.NewUserWithCredentials(input.Name, input.Email, hash, input.BirthDate)
	if err != nil {
		return nil, err
	}

	if err := uc.userRepo.Save(ctx, user); err != nil {
		return nil, err
	}

	if err := uc.publisher.Publish(ctx, port.Event{
		Type:    "user.registered",
		Payload: user,
	}); err != nil {
		port.LoggerFromContext(ctx).Error(ctx, "failed to publish user.registered event",
			"event", "user.registered", "user_id", user.ID, "error", err)
	}

	return &RegisterUserOutput{ID: user.ID}, nil
}
