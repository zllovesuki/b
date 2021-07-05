package fast

import (
	"context"
	"encoding/binary"
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
		ex, err := ttlExceeded(f)
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

	if err := writeTTL(file, ttl); err != nil {
		return nil, err
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
	ex, err := ttlExceeded(file)
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

// TODO(zllovesuki): formalized the on-disk format as a specification
func writeTTL(f *os.File, ttl time.Duration) error {
	head := make([]byte, 15+8) // 15 bytes for when it was created, 8 bytes for ttl
	now, err := time.Now().UTC().MarshalBinary()
	if err != nil {
		return errors.Wrap(err, "error marshalling time into binary")
	}

	copy(head[:15], now)
	binary.LittleEndian.PutUint64(head[15:23], uint64(ttl))

	if _, err := f.Write(head); err != nil {
		return errors.Wrap(err, "cannot write expiration data")
	}

	return nil
}

func ttlExceeded(f *os.File) (bool, error) {
	head := make([]byte, 15+8)
	if _, err := f.Read(head); err != nil {
		return false, errors.Wrap(err, "cannot read expiration data")
	}

	ttl := int64(binary.LittleEndian.Uint64(head[15:23]))

	if ttl == 0 {
		return false, nil
	}

	var ref time.Time
	if err := ref.UnmarshalBinary(head[:15]); err != nil {
		return false, errors.Wrap(err, "error unmarshalling binary into time")
	}

	return time.Now().After(ref.Add(time.Duration(ttl))), nil
}
