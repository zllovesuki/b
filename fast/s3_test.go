package fast

import (
	"context"
	"crypto/rand"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/apptest"
)

func getS3Fixtures(t *testing.T) *S3FastBackend {
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
	return b
}

func TestS3FastBackend(t *testing.T) {
	b := getS3Fixtures(t)

	apptest.TestFastBackend(t, b)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	t.Run("get outside of ttl should expire", func(t *testing.T) {
		b := getS3Fixtures(t)
		reader := apptest.GetReaderFn(t)

		key := "out-of-ttl"
		ttl := time.Second

		_, err := b.SaveTTL(ctx, key, reader(), ttl/2)
		require.NoError(t, err)

		<-time.After(ttl)

		_, err = b.Retrieve(ctx, key)
		require.ErrorIs(t, err, app.ErrNotFound)

		// ensure that we delete on access
		_, err = b.mc.StatObject(ctx, b.config.Bucket, key, minio.StatObjectOptions{})
		require.Error(t, err)
		resp := minio.ToErrorResponse(err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("should remove failed partial uploads", func(t *testing.T) {
		b := getS3Fixtures(t)

		r, w := io.Pipe()

		key := "remove-partial"
		done := make(chan struct{})
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			_, err := b.Save(ctx, key, r)
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
		case x := <-b.mc.ListIncompleteUploads(ctx, b.config.Bucket, "", true):
			require.Empty(t, x.Key, "received incomplete uploads")
		case <-time.After(time.Second * 5):
		}
	})
}

func TestS3Delete(t *testing.T) {
	b := getS3Fixtures(t)

	apptest.TestRemovableFastBackend(t, b)
}
