package port

import (
	"context"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type UserRepository interface {
	Save(ctx context.Context, user *entity.User) error
	FindByID(ctx context.Context, id string) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}
