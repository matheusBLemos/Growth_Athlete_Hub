package usecase

import "errors"

var ErrDuplicateActivity = errors.New("activity with this external ID already exists")
