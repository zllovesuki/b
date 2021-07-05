package fast

import (
	"context"
	"io"
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
	file *os.File
}

func getFixtures(t *testing.T) (*testDependencies, func()) {
	b, err := NewFileFastBackend(p)
	require.NoError(t, err)

	file, err := os.Open(filepath.Join("fixtures", "image.jpg"))
	require.NoError(t, err)

	return &testDependencies{
			f:    b,
			file: file,
		}, func() {
			os.RemoveAll(p)
			file.Close()
		}
}

func TestFileFastBackend(t *testing.T) {
	t.Run("happy path - save and retrieve", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := "happy"

		w, err := dep.f.Save(context.Background(), key)
		require.NoError(t, err)
		defer w.Close()

		src, err := ioutil.ReadAll(dep.file)
		require.NoError(t, err)

		_, err = dep.file.Seek(0, 0)
		require.NoError(t, err)

		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		r, err := dep.f.Retrieve(context.Background(), key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, src, saved)
	})

	t.Run("same identifier on save should conflict", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := "conflicting"

		w, err := dep.f.Save(context.Background(), key)
		require.NoError(t, err)
		defer w.Close()
		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		_, err = dep.f.Save(context.Background(), key)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save with ttl should work on retrieve", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := "save-with-ttl"
		ttl := time.Second

		w, err := dep.f.SaveTTL(context.Background(), key, ttl*2)
		require.NoError(t, err)
		defer w.Close()

		src, err := ioutil.ReadAll(dep.file)
		require.NoError(t, err)

		_, err = dep.file.Seek(0, 0)
		require.NoError(t, err)

		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		<-time.After(ttl)

		r, err := dep.f.Retrieve(context.Background(), key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, src, saved)
	})

	t.Run("save within ttl should conflict", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := "save-with-ttl-conflict"
		ttl := time.Second

		w, err := dep.f.SaveTTL(context.Background(), key, ttl*2)
		require.NoError(t, err)
		defer w.Close()

		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		_, err = dep.f.SaveTTL(context.Background(), key, ttl*2)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save outside of ttl should not conflict", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := "save-past-ttl"
		ttl := time.Second

		w, err := dep.f.SaveTTL(context.Background(), key, ttl/2)
		require.NoError(t, err)
		defer w.Close()

		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		_, err = dep.file.Seek(0, 0)
		require.NoError(t, err)

		<-time.After(ttl)

		w, err = dep.f.SaveTTL(context.Background(), key, ttl*2)
		require.NoError(t, err)
		defer w.Close()

		src, err := ioutil.ReadAll(dep.file)
		require.NoError(t, err)

		_, err = dep.file.Seek(0, 0)
		require.NoError(t, err)

		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		<-time.After(ttl)

		r, err := dep.f.Retrieve(context.Background(), key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, src, saved)
	})

	t.Run("get outside of ttl should not found", func(t *testing.T) {
		dep, clean := getFixtures(t)
		defer clean()

		key := "get-past-ttl"
		ttl := time.Second

		w, err := dep.f.SaveTTL(context.Background(), key, ttl/2)
		require.NoError(t, err)
		defer w.Close()

		_, err = io.Copy(w, dep.file)
		require.NoError(t, err)

		<-time.After(ttl)

		_, err = dep.f.Retrieve(context.Background(), key)
		require.ErrorIs(t, err, app.ErrNotFound)
	})
}

func TestFileDelete(t *testing.T) {
	dep, clean := getFixtures(t)
	defer clean()

	key := "happy"

	w, err := dep.f.Save(context.Background(), key)
	require.NoError(t, err)

	_, err = io.Copy(w, dep.file)
	require.NoError(t, err)
	w.Close()

	err = dep.f.Delete(context.Background(), key)
	require.NoError(t, err)

	_, err = dep.f.Retrieve(context.Background(), key)
	require.ErrorIs(t, err, app.ErrNotFound)
}
