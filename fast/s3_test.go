package fast

import (
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
)

type s3TestDependencies struct {
	f    *S3FastBackend
	file func() *os.File
}

func getS3Fixtures(t *testing.T) (*s3TestDependencies, func()) {
	b, err := NewS3FastBackend(S3Config{
		Bucket:         "testing",
		Endpoint:       "127.0.0.1:9000",
		Region:         "us-east-1",
		DisableSSL:     true,
		ForcePathStyle: true,
		AccessKey:      "minioadmin",
		AccessSecret:   "minioadmin",
	})
	require.NoError(t, err)

	return &s3TestDependencies{
			f: b,
			file: func() *os.File {
				file, err := os.Open(filepath.Join("fixtures", "image.jpg"))
				require.NoError(t, err)
				return file
			},
		}, func() {
		}
}

func TestS3FastBackend(t *testing.T) {
	t.Run("happy path - save and retrieve", func(t *testing.T) {
		dep, clean := getS3Fixtures(t)
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
		dep, clean := getS3Fixtures(t)
		defer clean()

		key := randomString(16)

		_, err := dep.f.Save(context.Background(), key, dep.file())
		require.NoError(t, err)

		_, err = dep.f.Save(context.Background(), key, dep.file())
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save with ttl should work on retrieve", func(t *testing.T) {
		dep, clean := getS3Fixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Second

		src, err := ioutil.ReadAll(dep.file())
		require.NoError(t, err)

		written, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl*5)
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
		dep, clean := getS3Fixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Hour

		_, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl)
		require.NoError(t, err)

		_, err = dep.f.SaveTTL(context.Background(), key, dep.file(), ttl/2)
		require.ErrorIs(t, err, app.ErrConflict)
	})

	t.Run("save outside of ttl should not conflict", func(t *testing.T) {
		dep, clean := getS3Fixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Second

		_, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl/2)
		require.NoError(t, err)

		<-time.After(ttl)

		written, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl*5)
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

		// ensure that the file still exists if not yet expired
		_, err = dep.f.mc.StatObject(context.Background(), dep.f.config.Bucket, key, minio.StatObjectOptions{})
		require.NoError(t, err)
	})

	t.Run("get outside of ttl should expire", func(t *testing.T) {
		dep, clean := getS3Fixtures(t)
		defer clean()

		key := randomString(16)
		ttl := time.Second

		_, err := dep.f.SaveTTL(context.Background(), key, dep.file(), ttl/2)
		require.NoError(t, err)

		<-time.After(ttl)

		_, err = dep.f.Retrieve(context.Background(), key)
		require.ErrorIs(t, err, app.ErrExpired)

		// ensure that we delete on access
		_, err = dep.f.mc.StatObject(context.Background(), dep.f.config.Bucket, key, minio.StatObjectOptions{})
		require.Error(t, err)
		resp := minio.ToErrorResponse(err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("should remove failed partial uploads", func(t *testing.T) {
		dep, clean := getS3Fixtures(t)
		defer clean()

		r, w := io.Pipe()

		key := randomString(16)
		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			_, err := dep.f.Save(ctx, key, r)
			require.Error(t, err)
			done <- struct{}{}
		}()

		go func() {
			_, err := io.Copy(w, app.NewCtxReader(ctx, rand.Reader))
			require.Error(t, err)
		}()

		// simulate closed pipe
		<-time.After(time.Second * 5)
		r.Close()

		<-done

		// check if we have partial uploads
		select {
		case x := <-dep.f.mc.ListIncompleteUploads(ctx, dep.f.config.Bucket, "", true):
			require.Empty(t, x.Key, "received incomplete uploads")
		case <-time.After(time.Second * 5):
		}
	})
}

func TestS3Delete(t *testing.T) {
	dep, clean := getS3Fixtures(t)
	defer clean()

	key := randomString(16)

	written, err := dep.f.Save(context.Background(), key, dep.file())
	require.NoError(t, err)

	src, err := ioutil.ReadAll(dep.file())
	require.NoError(t, err)
	require.Equal(t, written, int64(len(src)))

	err = dep.f.Delete(context.Background(), key)
	require.NoError(t, err)

	_, err = dep.f.Retrieve(context.Background(), key)
	require.ErrorIs(t, err, app.ErrNotFound)

	// deleting non-existent identifier should be noop
	err = dep.f.Delete(context.Background(), randomString(16))
	require.NoError(t, err)
}
