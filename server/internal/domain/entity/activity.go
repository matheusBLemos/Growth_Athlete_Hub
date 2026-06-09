package entity

import (
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type Activity struct {
	ID           string
	UserID       string
	Type         valueobject.ActivityType
	Date         time.Time
	Duration     time.Duration
	AvgHeartRate int
	ExternalID   string
	CreatedAt    time.Time
}

func NewActivity(userID string, activityType valueobject.ActivityType, date time.Time, duration time.Duration, avgHR int, externalID string) (*Activity, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	if duration <= 0 {
		return nil, ErrInvalidDuration
	}
	if date.After(time.Now()) {
		return nil, ErrActivityDateFuture
	}
	if avgHR != 0 && (avgHR < 30 || avgHR > 220) {
		return nil, ErrHROutOfRange
	}

	return &Activity{
		ID:           generateID(),
		UserID:       userID,
		Type:         activityType,
		Date:         date,
		Duration:     duration,
		AvgHeartRate: avgHR,
		ExternalID:   externalID,
		CreatedAt:    time.Now(),
	}, nil
}
