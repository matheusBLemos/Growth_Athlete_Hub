package usecase

import (
	"context"
	"log"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type RegisterActivityInput struct {
	UserID       string
	ActivityType string
	Date         time.Time
	Duration     time.Duration
	AvgHeartRate int
	ExternalID   string
}

type RegisterActivityOutput struct {
	ID string
}

type RegisterActivity struct {
	activityRepo port.ActivityRepository
	publisher    port.EventPublisher
}

func NewRegisterActivity(repo port.ActivityRepository, pub port.EventPublisher) *RegisterActivity {
	return &RegisterActivity{
		activityRepo: repo,
		publisher:    pub,
	}
}

func (uc *RegisterActivity) Execute(ctx context.Context, input RegisterActivityInput) (*RegisterActivityOutput, error) {
	actType, err := valueobject.NewActivityType(input.ActivityType)
	if err != nil {
		return nil, err
	}

	if input.ExternalID != "" {
		existing, err := uc.activityRepo.FindByExternalID(ctx, input.ExternalID)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return nil, ErrDuplicateActivity
		}
	}

	activity, err := entity.NewActivity(input.UserID, actType, input.Date, input.Duration, input.AvgHeartRate, input.ExternalID)
	if err != nil {
		return nil, err
	}

	if err := uc.activityRepo.Save(ctx, activity); err != nil {
		return nil, err
	}

	if err := uc.publisher.Publish(ctx, port.Event{
		Type:    "activity.registered",
		Payload: activity,
	}); err != nil {
		log.Printf("failed to publish activity.registered event: %v", err)
	}

	return &RegisterActivityOutput{ID: activity.ID}, nil
}
