package cache

import (
	"errors"
)

var (
	// ErrNoSuchKey is returned when a key is not found
	ErrNoSuchKey = errors.New("no such key")
)

// Cache is the interface that
type Cache interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Keys() ([]string, error)
	Delete(string) error
}
