package entity

import "errors"

var (
	ErrEmptyName          = errors.New("name must not be empty")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrBirthDateFuture    = errors.New("birth date must be in the past")
	ErrEmptyUserID        = errors.New("user ID must not be empty")
	ErrInvalidDuration    = errors.New("duration must be greater than zero")
	ErrActivityDateFuture = errors.New("activity date must not be in the future")
	ErrHROutOfRange       = errors.New("average heart rate must be between 30 and 220 bpm")
	ErrMetricOutOfRange   = errors.New("metric value is out of acceptable range")
	ErrEmptyDate          = errors.New("date must not be empty")
	ErrEmptyMessage       = errors.New("message must not be empty")
)
