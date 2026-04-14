package alerterr

import "errors"

var (
	ErrAlertNotFound     = errors.New("alert not found")
	ErrInvalidAlertInput = errors.New("invalid alert input")
	ErrAlertConflict     = errors.New("alert conflict")
)
