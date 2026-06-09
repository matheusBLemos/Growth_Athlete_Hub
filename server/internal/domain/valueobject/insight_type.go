package valueobject

import "errors"

var ErrInvalidInsightType = errors.New("invalid insight type")

type InsightType string

const (
	InsightTypeHRVDrop        InsightType = "hrv_drop"
	InsightTypeRestingHRHigh  InsightType = "resting_hr_high"
	InsightTypeSleepDeficit   InsightType = "sleep_deficit"
	InsightTypeOvertraining   InsightType = "overtraining"
	InsightTypeUndertraining  InsightType = "undertraining"
	InsightTypeRecoveryNeeded InsightType = "recovery_needed"
)

var validInsightTypes = map[InsightType]bool{
	InsightTypeHRVDrop:        true,
	InsightTypeRestingHRHigh:  true,
	InsightTypeSleepDeficit:   true,
	InsightTypeOvertraining:   true,
	InsightTypeUndertraining:  true,
	InsightTypeRecoveryNeeded: true,
}

func NewInsightType(s string) (InsightType, error) {
	it := InsightType(s)
	if !validInsightTypes[it] {
		return "", ErrInvalidInsightType
	}
	return it, nil
}
