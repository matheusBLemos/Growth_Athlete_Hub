package valueobject

import "errors"

var ErrInvalidMetricType = errors.New("invalid metric type")

type MetricType string

const (
	MetricTypeHRV           MetricType = "hrv"
	MetricTypeRestingHR     MetricType = "resting_hr"
	MetricTypeSleepDuration MetricType = "sleep_duration"
	MetricTypeSleepQuality  MetricType = "sleep_quality"
	MetricTypeWeight        MetricType = "weight"
	MetricTypeBodyFat       MetricType = "body_fat"
	MetricTypeCaloriesIn    MetricType = "calories_in"
	MetricTypeCaloriesOut   MetricType = "calories_out"
	MetricTypeTrainingLoad  MetricType = "training_load"
)

var validMetricTypes = map[MetricType]bool{
	MetricTypeHRV:           true,
	MetricTypeRestingHR:     true,
	MetricTypeSleepDuration: true,
	MetricTypeSleepQuality:  true,
	MetricTypeWeight:        true,
	MetricTypeBodyFat:       true,
	MetricTypeCaloriesIn:    true,
	MetricTypeCaloriesOut:   true,
	MetricTypeTrainingLoad:  true,
}

func NewMetricType(s string) (MetricType, error) {
	mt := MetricType(s)
	if !validMetricTypes[mt] {
		return "", ErrInvalidMetricType
	}
	return mt, nil
}
