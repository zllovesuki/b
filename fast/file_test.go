package fast

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/apptest"
)

var p = filepath.Join(os.TempDir(), "b-fast")

func getFixtures(t *testing.T) (*FileFastBackend, func()) {
	b, err := NewFileFastBackend(p)
	require.NoError(t, err)

	return b, func() {
		os.RemoveAll(p)
	}
}

func TestFileFastBackend(t *testing.T) {
	b, clean := getFixtures(t)
	defer clean()

	apptest.TestFastBackend(t, b)

}

func TestRemoveOnAccess(t *testing.T) {
	b, clean := getFixtures(t)
	defer clean()

	key := "remove-on-access"
	reader := apptest.GetReaderFn(t)
	ttl := time.Second

	path := filepath.Join(p, key)

	_, err := b.SaveTTL(context.Background(), key, reader(), ttl/2)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.NoError(t, err)

	<-time.After(ttl)

	_, err = b.Retrieve(context.Background(), key)
	require.ErrorIs(t, err, app.ErrNotFound)

	// ensure that we delete on access
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestFileDelete(t *testing.T) {
	b, clean := getFixtures(t)
	defer clean()

	apptest.TestRemovableFastBackend(t, b)
}
