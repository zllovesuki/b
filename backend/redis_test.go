package backend

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
)

func getRedisFixtures(t *testing.T) (*RedisBackend, func()) {
	b, err := NewBasicRedisBackend("127.0.0.1:6379")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = b.cli.Ping(ctx).Err()
	require.NoError(t, err)

	rand.Seed(time.Now().Unix())

	return b, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := b.cli.FlushAll(ctx).Err()
		require.NoError(t, err)
		err = b.cli.Close()
		require.NoError(t, err)
	}
}

func TestRedisBackend(t *testing.T) {
	t.Run("save should return no error", func(t *testing.T) {
		b, cleanup := getRedisFixtures(t)
		defer cleanup()

		err := b.Save(context.Background(), "hi", []byte("h"))
		require.NoError(t, err)
	})

	t.Run("save should return error on conflict", func(t *testing.T) {
		b, cleanup := getRedisFixtures(t)
		defer cleanup()

		key := "hi"

		err := b.Save(context.Background(), key, []byte("h"))
		require.NoError(t, err)

		err = b.Save(context.Background(), key, []byte("h"))
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save ttl should work and expire when retrieve", func(t *testing.T) {
		b, cleanup := getRedisFixtures(t)
		defer cleanup()

		key := "hi"
		wait := time.Second

		err := b.SaveTTL(context.Background(), key, []byte("h"), wait/2)
		require.NoError(t, err)

		<-time.After(wait)

		ret, err := b.Retrieve(context.Background(), key)
		require.ErrorIs(t, err, app.ErrNotFound)
		require.Nil(t, ret)
	})

	t.Run("save ttl should return conflict if within ttl", func(t *testing.T) {
		b, cleanup := getRedisFixtures(t)
		defer cleanup()

		key := "hi"
		wait := time.Hour

		err := b.SaveTTL(context.Background(), key, []byte("h"), wait)
		require.NoError(t, err)

		err = b.Save(context.Background(), key, []byte("h"))
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("retrieve should return what we saved", func(t *testing.T) {
		b, cleanup := getRedisFixtures(t)
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
		b, cleanup := getRedisFixtures(t)
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

func TestRedisDelete(t *testing.T) {
	b, cleanup := getRedisFixtures(t)
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
