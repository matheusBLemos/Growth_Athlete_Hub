package usecase

import (
	"context"
	"strings"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

type LoginUserInput struct {
	Email    string
	Password string
}

type LoginUserOutput struct {
	Token string
}

type LoginUser struct {
	userRepo port.UserRepository
	hasher   port.PasswordHasher
	issuer   port.TokenIssuer
}

func NewLoginUser(repo port.UserRepository, hasher port.PasswordHasher, issuer port.TokenIssuer) *LoginUser {
	return &LoginUser{
		userRepo: repo,
		hasher:   hasher,
		issuer:   issuer,
	}
}

func (uc *LoginUser) Execute(ctx context.Context, input LoginUserInput) (*LoginUserOutput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	user, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	// Não vaza qual parte falhou (e-mail inexistente vs senha incorreta).
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if err := uc.hasher.Compare(user.PasswordHash, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := uc.issuer.Issue(user.ID)
	if err != nil {
		return nil, err
	}

	return &LoginUserOutput{Token: token}, nil
}
