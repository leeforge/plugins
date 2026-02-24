package shared

import "errors"

var (
	ErrNilAppContext      = errors.New("ou plugin: app context is nil")
	ErrNilServiceRegistry = errors.New("ou plugin: service registry is nil")
)
