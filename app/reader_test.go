package app

import (
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReaderCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*500)
	defer cancel()

	go func() {
		_, err := io.Copy(io.Discard, NewCtxReader(ctx, rand.Reader))

		require.Error(t, err)
	}()

	<-time.After(time.Second)
}
