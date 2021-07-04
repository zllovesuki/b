package fast

import (
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/djherbis/times"
	"github.com/pkg/errors"
	"github.com/zllovesuki/b/app"
)

// FileFastBackend is a file-backed app.FastBackend implementation with support for TTL
type FileFastBackend struct {
	dataDir string
}

var _ app.FastBackend = &FileFastBackend{}

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

	_, err := os.Stat(p)
	if !errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(p, os.O_RDONLY, 0600)
		if err != nil {
			return nil, errors.Wrap(err, "cannot open file to check for expiration")
		}
		defer f.Close()
		ttl, err := getTTL(f)
		if err != nil {
			return nil, errors.Wrap(err, "error reading file to check for expiration")
		}
		if ttl == 0 {
			return nil, app.ErrConflict
		}
		ex, err := ttlExceeded(f, ttl)
		if err != nil {
			return nil, errors.Wrap(err, "error checking ttl of the file")
		}
		if !ex {
			return nil, app.ErrConflict
		}
	}

	file, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}

	// save expiration data at the head
	exp := make([]byte, 8)
	binary.LittleEndian.PutUint64(exp, uint64(ttl))
	if _, err := file.Write(exp); err != nil {
		return nil, errors.Wrap(err, "cannot write expiration data")
	}

	return file, nil
}

func (f *FileFastBackend) Retrieve(c context.Context, identifier string) (io.ReadCloser, error) {
	p := filepath.Join(f.dataDir, identifier)

	file, err := os.OpenFile(p, os.O_RDONLY, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "cannot open file")
	}

	_, err = file.Stat()
	if errors.Is(err, os.ErrNotExist) {
		return nil, app.ErrNotFound
	}
	if err != nil {
		return nil, errors.Wrap(err, "cannot stat file")
	}

	// read expiration data back
	ttl, err := getTTL(file)
	if err != nil {
		return nil, errors.Wrap(err, "error reading file to check for expiration")
	}

	if ttl == 0 {
		return file, nil
	}

	ex, err := ttlExceeded(file, ttl)
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

func getTTL(f *os.File) (int64, error) {
	exp := make([]byte, 8)
	if _, err := f.Read(exp); err != nil {
		return 0, errors.Wrap(err, "cannot read expiration data")
	}
	return int64(binary.LittleEndian.Uint64(exp)), nil
}

func ttlExceeded(f *os.File, ttl int64) (bool, error) {
	t, err := times.StatFile(f)
	if err != nil {
		return false, errors.Wrap(err, "unable to get file metadata")
	}

	var ref time.Time
	if t.HasBirthTime() {
		ref = t.BirthTime()
	} else if t.HasChangeTime() {
		ref = t.ChangeTime()
	}

	return !ref.IsZero() && time.Now().After(ref.Add(time.Duration(ttl))), nil
}
