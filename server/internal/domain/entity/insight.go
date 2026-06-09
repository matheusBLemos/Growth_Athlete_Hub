package entity

import (
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type Insight struct {
	ID        string
	UserID    string
	Type      valueobject.InsightType
	Severity  valueobject.Severity
	Message   string
	Date      time.Time
	CreatedAt time.Time
}

func NewInsight(userID string, insightType valueobject.InsightType, severity valueobject.Severity, message string, date time.Time) (*Insight, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	if message == "" {
		return nil, ErrEmptyMessage
	}

	return &Insight{
		ID:        generateID(),
		UserID:    userID,
		Type:      insightType,
		Severity:  severity,
		Message:   message,
		Date:      date,
		CreatedAt: time.Now(),
	}, nil
}
