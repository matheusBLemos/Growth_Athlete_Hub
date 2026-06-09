package entity

import (
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

type Metric struct {
	ID        string
	UserID    string
	Type      valueobject.MetricType
	Value     float64
	Date      time.Time
	CreatedAt time.Time
}

var metricRanges = map[valueobject.MetricType][2]float64{
	valueobject.MetricTypeHRV:           {0, 300},
	valueobject.MetricTypeRestingHR:     {30, 120},
	valueobject.MetricTypeSleepDuration: {0, 24},
	valueobject.MetricTypeSleepQuality:  {0, 100},
	valueobject.MetricTypeWeight:        {20, 300},
	valueobject.MetricTypeBodyFat:       {0, 100},
	valueobject.MetricTypeCaloriesIn:    {0, 20000},
	valueobject.MetricTypeCaloriesOut:   {0, 20000},
	valueobject.MetricTypeTrainingLoad:  {0, 10000},
}

func NewMetric(userID string, metricType valueobject.MetricType, value float64, date time.Time) (*Metric, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	if date.IsZero() {
		return nil, ErrEmptyDate
	}
	if !metricType.IsValid() {
		return nil, valueobject.ErrInvalidMetricType
	}
	if r, ok := metricRanges[metricType]; ok {
		if value < r[0] || value > r[1] {
			return nil, ErrMetricOutOfRange
		}
	}

	return &Metric{
		ID:        generateID(),
		UserID:    userID,
		Type:      metricType,
		Value:     value,
		Date:      date,
		CreatedAt: time.Now(),
	}, nil
}
