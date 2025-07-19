package runtime

import "errors"

var (
	// Package creation errors
	ErrNilModule  = errors.New("module cannot be nil")
	ErrNilSchema  = errors.New("schema cannot be nil")
	ErrNilConfig  = errors.New("config cannot be nil")
	ErrNoServices = errors.New("schema must contain at least one service")

	// Validation errors
	ErrMethodNotFound = errors.New("method not found in service schema")
	ErrInvalidInput   = errors.New("invalid input for method")
)
