package app

//go:generate mockgen -destination=backend_mocks.go -package=app github.com/zllovesuki/b/app Backend,FastBackend,Removable,RemovableBackend,RemovableFastBackend

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
	// SaveTTL will persist the data but a defined expiration time
	SaveTTL(c context.Context, identifier string, data []byte, ttl time.Duration) error
	// Retrieve gets the persisted data back
	Retrieve(c context.Context, identifier string) ([]byte, error)
	// Close releases resources before exit
	Close() error
}

// FastBackend is similar to Backend, except that it utilizes io.ReadCloser to minimize buffering
type FastBackend interface {
	SaveTTL(c context.Context, identifier string, r io.ReadCloser, ttl time.Duration) (int64, error)
	Retrieve(c context.Context, identifier string) (io.ReadCloser, error)
	Close() error
}

// Removable is used to remove underlying resources, usually in internal tools
type Removable interface {
	Delete(c context.Context, identifier string) error
}

type RemovableBackend interface {
	Backend
	Removable
}

type RemovableFastBackend interface {
	FastBackend
	Removable
}
