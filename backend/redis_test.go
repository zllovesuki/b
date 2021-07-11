package backend

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/apptest"
)

func getRedisFixtures(t *testing.T) (*RedisBackend, func()) {
	b, err := NewRedisBackend("127.0.0.1:6379")
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
	b, cleanup := getRedisFixtures(t)
	defer cleanup()

	apptest.TestBackend(t, b)
}

func TestRedisDelete(t *testing.T) {
	b, cleanup := getRedisFixtures(t)
	defer cleanup()

	apptest.TestRemovableBackend(t, b)
}
