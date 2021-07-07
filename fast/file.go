package fast

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/zllovesuki/b/app"
)

// FileFastBackend is a file-backed app.FastBackend implementation with support for TTL
type FileFastBackend struct {
	dataDir string
}

var _ app.FastBackend = &FileFastBackend{}
var _ app.Removable = &FileFastBackend{}

func NewFileFastBackend(dataDir string) (*FileFastBackend, error) {
	if dataDir == "" {
		return nil, errors.New("dataDir cannot be empty")
	}

	info, err := os.Stat(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(dataDir, 0750); err != nil {
			return nil, errors.New("error creating dataDir")
		}
	} else {
		if !info.IsDir() {
			return nil, errors.New("dataDir is not a directory")
		}
	}

	return &FileFastBackend{
		dataDir: dataDir,
	}, nil
}

func (f *FileFastBackend) Save(c context.Context, identifier string) (io.WriteCloser, error) {
	return f.SaveTTL(c, identifier, 0)
}

func (f *FileFastBackend) SaveTTL(c context.Context, identifier string, ttl time.Duration) (io.WriteCloser, error) {
	p := filepath.Join(f.dataDir, identifier)

	exist := true

	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			exist = false
		} else {
			return nil, errors.Wrap(err, "testing file existence")
		}
	}

	if exist {
		r, err := os.OpenFile(p, os.O_RDONLY, 0600)
		if err != nil {
			return nil, errors.Wrap(err, "opening file for ttl checking")
		}
		defer r.Close()

		ex, err := app.TTLExceeded(r)
		if err != nil {
			return nil, errors.Wrap(err, "checking ttl of the file")
		}
		if !ex {
			return nil, app.ErrConflict
		}
	}

	// overwrite file if ttl exceeded, or just a new file in general
	w, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}

	if err := app.WriteTTL(w, ttl); err != nil {
		return nil, err
	}

	return w, nil
}

func (f *FileFastBackend) Retrieve(c context.Context, identifier string) (io.ReadCloser, error) {
	p := filepath.Join(f.dataDir, identifier)

	file, err := os.OpenFile(p, os.O_RDONLY, 0600)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, app.ErrNotFound
		}
		return nil, errors.Wrap(err, "cannot open file")
	}

	ex, err := app.TTLExceeded(file)
	if err != nil {
		return nil, errors.Wrap(err, "error checking ttl of the file")
	}

	if ex {
		// compaction on access
		defer os.Remove(p)
		return nil, app.ErrNotFound
	}

	return file, nil
}

func (f *FileFastBackend) Delete(c context.Context, identifier string) error {
	p := filepath.Join(f.dataDir, identifier)

	return os.Remove(p)
}
