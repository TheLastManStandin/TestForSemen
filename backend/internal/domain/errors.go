package domain

import "errors"

// ErrNotFound is returned when an entity does not exist or is outside the caller's scope.
var ErrNotFound = errors.New("не найдено")

// ValidationError describes invalid input for a single field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// Invalid builds a *ValidationError.
func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
