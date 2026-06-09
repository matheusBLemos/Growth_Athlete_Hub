package valueobject

import "errors"

var ErrInvalidActivityType = errors.New("invalid activity type")

type ActivityType string

const (
	ActivityTypeRunning       ActivityType = "running"
	ActivityTypeCycling       ActivityType = "cycling"
	ActivityTypeSwimming      ActivityType = "swimming"
	ActivityTypeWeightlifting ActivityType = "weightlifting"
	ActivityTypeYoga          ActivityType = "yoga"
	ActivityTypeHiking        ActivityType = "hiking"
	ActivityTypeCrossfit      ActivityType = "crossfit"
	ActivityTypeOther         ActivityType = "other"
)

var validActivityTypes = map[ActivityType]bool{
	ActivityTypeRunning:       true,
	ActivityTypeCycling:       true,
	ActivityTypeSwimming:      true,
	ActivityTypeWeightlifting: true,
	ActivityTypeYoga:          true,
	ActivityTypeHiking:        true,
	ActivityTypeCrossfit:      true,
	ActivityTypeOther:         true,
}

func NewActivityType(s string) (ActivityType, error) {
	at := ActivityType(s)
	if !validActivityTypes[at] {
		return "", ErrInvalidActivityType
	}
	return at, nil
}
