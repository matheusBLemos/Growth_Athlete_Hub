package port

import (
	"context"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type ActivityRepository interface {
	Save(ctx context.Context, activity *entity.Activity) error
	FindByID(ctx context.Context, id string) (*entity.Activity, error)
	FindByUserID(ctx context.Context, userID string, from, to time.Time) ([]*entity.Activity, error)
	FindByExternalID(ctx context.Context, externalID string) (*entity.Activity, error)
}
