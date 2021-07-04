package app

//go:generate mockgen -destination=backend_mocks.go -package=app github.com/zllovesuki/b/app Backend,FastBackend

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Define errors used by the application
var (
	ErrNotFound = fmt.Errorf("not found")
	ErrConflict = fmt.Errorf("conflict identifier")
)

// Backend is used to store and later retrieve our documents (links, files, etc)
type Backend interface {
	// Save will persist the data with the given identifier
	Save(c context.Context, identifier string, data []byte) error
	// SaveTTL will persist the data but a defined expiration time
	SaveTTL(c context.Context, identifier string, data []byte, ttl time.Duration) error
	// Retrieve gets the persisted data back
	Retrieve(c context.Context, identifier string) ([]byte, error)
}

// FastBackend is similar to Backend, except that it utilizes io.ReadCloser/io.WriteCloser
// to minimize buffering
type FastBackend interface {
	Save(c context.Context, identifier string) (io.WriteCloser, error)
	SaveTTL(c context.Context, identifier string, ttl time.Duration) (io.WriteCloser, error)
	Retrieve(c context.Context, identifier string) (io.ReadCloser, error)
}
