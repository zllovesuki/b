package apptest

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
)

type consistentBuffer struct {
	buf []byte
}

func GetReaderFn(t *testing.T) func() io.ReadCloser {
	buf := make([]byte, 10240)
	_, err := io.ReadFull(rand.Reader, buf)
	require.NoError(t, err)

	return func() io.ReadCloser {
		return io.NopCloser(bytes.NewReader(buf))
	}
}

func TestBackend(t *testing.T, b app.Backend) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("save should return no error", func(t *testing.T) {
		key := randomString(16)

		err := b.SaveTTL(ctx, key, []byte("h"), 0)
		require.NoError(t, err)
	})

	t.Run("save should return error on conflict", func(t *testing.T) {
		key := randomString(16)

		err := b.SaveTTL(ctx, key, []byte("h"), 0)
		require.NoError(t, err)

		err = b.SaveTTL(ctx, key, []byte("h"), 0)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save ttl should work and expire when retrieve", func(t *testing.T) {
		key := randomString(16)
		wait := time.Second

		err := b.SaveTTL(ctx, key, []byte("h"), wait/2)
		require.NoError(t, err)

		<-time.After(wait)

		ret, err := b.Retrieve(ctx, key)
		require.ErrorIs(t, err, app.ErrNotFound)
		require.Nil(t, ret)
	})

	t.Run("save ttl should return conflict if within ttl", func(t *testing.T) {
		key := randomString(16)
		wait := time.Hour

		err := b.SaveTTL(ctx, key, []byte("h"), wait)
		require.NoError(t, err)

		err = b.SaveTTL(ctx, key, []byte("h"), wait)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("retrieve should return what we saved", func(t *testing.T) {
		key := randomString(16)

		buf := make([]byte, 8)
		_, err := rand.Read(buf)
		require.NoError(t, err)

		err = b.SaveTTL(ctx, key, buf, 0)
		require.NoError(t, err)

		ret, err := b.Retrieve(ctx, key)
		require.NoError(t, err)
		require.Equal(t, buf, ret)
	})

	t.Run("retrieve should return what we saved within expiration", func(t *testing.T) {
		key := randomString(16)

		buf := make([]byte, 8)
		_, err := rand.Read(buf)
		require.NoError(t, err)

		err = b.SaveTTL(ctx, key, buf, time.Hour)
		require.NoError(t, err)

		ret, err := b.Retrieve(ctx, key)
		require.NoError(t, err)
		require.Equal(t, buf, ret)
	})
}

func TestRemovableBackend(t *testing.T, b app.RemovableBackend) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("remove should work", func(t *testing.T) {
		key := randomString(16)

		buf := make([]byte, 8)
		_, err := rand.Read(buf)
		require.NoError(t, err)

		err = b.SaveTTL(ctx, key, buf, 0)
		require.NoError(t, err)

		err = b.Delete(ctx, key)
		require.NoError(t, err)

		_, err = b.Retrieve(ctx, key)
		require.ErrorIs(t, err, app.ErrNotFound)
	})

	t.Run("remove should be idempotent", func(t *testing.T) {
		key := randomString(16)

		err := b.Delete(ctx, key)
		require.NoError(t, err)
	})
}

func TestFastBackend(t *testing.T, b app.FastBackend) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("happy path - save and retrieve", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)

		src, err := ioutil.ReadAll(reader())
		require.NoError(t, err)

		written, err := b.SaveTTL(ctx, key, reader(), 0)
		require.NoError(t, err)

		r, err := b.Retrieve(ctx, key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, written, int64(len(saved)))
		require.Equal(t, src, saved)
	})

	t.Run("same identifier on save should conflict", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)

		_, err := b.SaveTTL(ctx, key, reader(), 0)
		require.NoError(t, err)

		_, err = b.SaveTTL(ctx, key, reader(), 0)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save with ttl should work on retrieve", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)
		ttl := time.Second

		src, err := ioutil.ReadAll(reader())
		require.NoError(t, err)

		written, err := b.SaveTTL(ctx, key, reader(), ttl*2)
		require.NoError(t, err)

		<-time.After(ttl)

		r, err := b.Retrieve(ctx, key)
		require.NoError(t, err)
		defer r.Close()
		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)

		require.Equal(t, written, int64(len(saved)))
		require.Equal(t, src, saved)
	})

	t.Run("save within ttl should conflict", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)
		ttl := time.Hour

		_, err := b.SaveTTL(ctx, key, reader(), ttl)
		require.NoError(t, err)

		_, err = b.SaveTTL(ctx, key, reader(), ttl/2)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save outside of ttl should not conflict", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)
		ttl := time.Second

		_, err := b.SaveTTL(ctx, key, reader(), ttl/2)
		require.NoError(t, err)

		<-time.After(ttl)

		written, err := b.SaveTTL(ctx, key, reader(), ttl*2)
		require.NoError(t, err)

		<-time.After(ttl)

		r, err := b.Retrieve(ctx, key)
		require.NoError(t, err)
		defer r.Close()

		saved, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		src, err := ioutil.ReadAll(reader())
		require.NoError(t, err)

		require.Equal(t, written, int64(len(saved)))
		require.Equal(t, src, saved)
	})

	t.Run("get outside of ttl should expire", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)
		ttl := time.Second

		_, err := b.SaveTTL(ctx, key, reader(), ttl/2)
		require.NoError(t, err)

		<-time.After(ttl)

		_, err = b.Retrieve(ctx, key)
		require.ErrorIs(t, err, app.ErrNotFound)
	})
}

func TestRemovableFastBackend(t *testing.T, b app.RemovableFastBackend) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	t.Run("remove should work", func(t *testing.T) {
		key := randomString(16)
		reader := GetReaderFn(t)

		_, err := b.SaveTTL(ctx, key, reader(), 0)
		require.NoError(t, err)

		err = b.Delete(ctx, key)
		require.NoError(t, err)

		_, err = b.Retrieve(ctx, key)
		require.ErrorIs(t, err, app.ErrNotFound)
	})

	t.Run("remove should be idempotent", func(t *testing.T) {
		key := randomString(16)

		err := b.Delete(ctx, key)
		require.NoError(t, err)
	})
}
