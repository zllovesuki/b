package fast

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
)

var p = filepath.Join(os.TempDir(), "b-fast")

type testDependencies struct {
	f    *FileFastBackend
	file func() *os.File
}

func getFixtures(t *testing.T) (*testDependencies, func()) {
	b, err := NewFileFastBackend(p)
	require.NoError(t, err)

	return &testDependencies{
			f: b,
			file: func() *os.File {
				file, err := os.Open(filepath.Join("fixtures", "image.jpg"))
				require.NoError(t, err)
				return file
			},
		}, func() {
			os.RemoveAll(p)
		}
}

func TestFileFastBackend(t *testing.T) {
	t.Run("happy path - save and retrieve", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := randomString(16)

		src, err := ioutil.ReadAll(dep.file())
		require.NoError(t, err)

		written, err := dep.f.Save(context.Background(), key, dep.file())
		require.NoError(t, err)

		r, err := dep.f.Retrieve(context.Background(), key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, written, int64(len(saved)))
		require.Equal(t, src, saved)
	})

	t.Run("same identifier on save should conflict", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := randomString(16)

		_, err := dep.f.Save(context.Background(), key, dep.file())
		require.NoError(t, err)

		_, err = dep.f.Save(context.Background(), key, dep.file())
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save with ttl should work on retrieve", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Second

		src, err := ioutil.ReadAll(dep.file())
		require.NoError(t, err)

		written, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl*2)
		require.NoError(t, err)

		<-time.After(ttl)

		r, err := dep.f.Retrieve(context.Background(), key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, written, int64(len(saved)))
		require.Equal(t, src, saved)
	})

	t.Run("save within ttl should conflict", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Hour

		_, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl)
		require.NoError(t, err)

		_, err = dep.f.SaveTTL(context.Background(), key, dep.file(), ttl/2)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save outside of ttl should not conflict", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Second

		_, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl/2)
		require.NoError(t, err)

		<-time.After(ttl)

		written, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl*2)
		require.NoError(t, err)

		<-time.After(ttl)

		r, err := dep.f.Retrieve(context.Background(), key)
		require.NoError(t, err)
		defer r.Close()

		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		src, err := ioutil.ReadAll(dep.file())
		require.NoError(t, err)

		require.Equal(t, written, int64(len(saved)))
		require.Equal(t, src, saved)
	})

	t.Run("get outside of ttl should expire", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Second

		path := filepath.Join(p, key)

		_, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl/2)
		require.NoError(t, err)

		_, err = os.Stat(path)
		require.NoError(t, err)

		<-time.After(ttl)

		_, err = dep.f.Retrieve(context.Background(), key)
		require.ErrorIs(t, err, app.ErrExpired)

		// ensure that we delete on access
		_, err = os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestFileDelete(t *testing.T) {
	dep, clean := getFixtures(t)
	defer clean()

	key := randomString(16)

	_, err := dep.f.Save(context.Background(), key, dep.file())
	require.NoError(t, err)

	err = dep.f.Delete(context.Background(), key)
	require.NoError(t, err)

	_, err = dep.f.Retrieve(context.Background(), key)
	require.ErrorIs(t, err, app.ErrNotFound)
}
