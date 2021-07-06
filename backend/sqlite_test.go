package backend

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
)

var p = filepath.Join(os.TempDir(), "b-sqlite-testing.db")

func getSQLiteFixtures(t *testing.T) (*SQLiteBackend, func()) {
	b, err := NewSQLiteBackend(p)
	require.NoError(t, err)

	rand.Seed(time.Now().Unix())

	return b, func() {
		db, err := b.db.DB()
		require.NoError(t, err)
		err = db.Close()
		require.NoError(t, err)
		os.Remove(p)
	}
}

func TestSQLiteBackend(t *testing.T) {
	t.Run("save should return no error", func(t *testing.T) {
		b, cleanup := getSQLiteFixtures(t)
		defer cleanup()

		err := b.Save(context.Background(), "hi", []byte("h"))
		require.NoError(t, err)
	})

	t.Run("save should return error on conflict", func(t *testing.T) {
		b, cleanup := getSQLiteFixtures(t)
		defer cleanup()

		key := "hi"

		err := b.Save(context.Background(), key, []byte("h"))
		require.NoError(t, err)

		err = b.Save(context.Background(), key, []byte("h"))
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save ttl should work and expire when retrieve", func(t *testing.T) {
		b, cleanup := getSQLiteFixtures(t)
		defer cleanup()

		key := "hi"
		wait := time.Second

		err := b.SaveTTL(context.Background(), key, []byte("h"), wait/2)
		require.NoError(t, err)

		<-time.After(wait)

		ret, err := b.Retrieve(context.Background(), key)
		require.ErrorIs(t, err, app.ErrExpired)
		require.Nil(t, ret)
	})

	t.Run("save ttl should return conflict if within ttl", func(t *testing.T) {
		b, cleanup := getSQLiteFixtures(t)
		defer cleanup()

		key := "hi"
		wait := time.Hour

		err := b.SaveTTL(context.Background(), key, []byte("h"), wait)
		require.NoError(t, err)

		err = b.Save(context.Background(), key, []byte("h"))
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("retrieve should return what we saved", func(t *testing.T) {
		b, cleanup := getSQLiteFixtures(t)
		defer cleanup()

		key := "hello"
		buf := make([]byte, 8)
		_, err := rand.Read(buf)
		require.NoError(t, err)

		err = b.Save(context.Background(), key, buf)
		require.NoError(t, err)

		ret, err := b.Retrieve(context.Background(), key)
		require.NoError(t, err)
		require.Equal(t, buf, ret)
	})

	t.Run("retrieve should return what we saved within expiration", func(t *testing.T) {
		b, cleanup := getSQLiteFixtures(t)
		defer cleanup()

		key := "hello"
		buf := make([]byte, 8)
		_, err := rand.Read(buf)
		require.NoError(t, err)

		err = b.SaveTTL(context.Background(), key, buf, time.Hour)
		require.NoError(t, err)

		ret, err := b.Retrieve(context.Background(), key)
		require.NoError(t, err)
		require.Equal(t, buf, ret)
	})

}

func TestSQLiteDelete(t *testing.T) {
	b, cleanup := getSQLiteFixtures(t)
	defer cleanup()

	key := "hello"
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	require.NoError(t, err)

	err = b.Save(context.Background(), key, buf)
	require.NoError(t, err)

	err = b.Delete(context.Background(), key)
	require.NoError(t, err)

	_, err = b.Retrieve(context.Background(), key)
	require.ErrorIs(t, err, app.ErrNotFound)
}
