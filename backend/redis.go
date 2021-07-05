package backend

import (
	"context"
	"time"

	"github.com/zllovesuki/b/app"

	redis "github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type RedisBackend struct {
	cli *redis.Client
}

var _ app.Backend = &RedisBackend{}
var _ app.Removable = &RedisBackend{}

// NewBasicRedisBackend returns a redis backed storage for the application
func NewBasicRedisBackend(url string) (*RedisBackend, error) {
	b := &RedisBackend{
		cli: redis.NewClient(&redis.Options{
			Addr: url,
		}),
	}
	context, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := b.cli.Ping(context).Err(); err != nil {
		return nil, err
	}

	return b, nil
}

func (b *RedisBackend) Save(c context.Context, identifier string, data []byte) error {
	return b.SaveTTL(c, identifier, data, 0)
}

func (b *RedisBackend) SaveTTL(c context.Context, identifier string, data []byte, ttl time.Duration) error {
	s, err := b.cli.SetNX(c, identifier, data, ttl).Result()
	if err != nil {
		return errors.Wrap(err, "unexpected error from redis when saving")
	}
	if !s {
		return app.ErrConflict
	}
	return nil
}

func (b *RedisBackend) Retrieve(c context.Context, identifier string) ([]byte, error) {
	ret, err := b.cli.Get(c, identifier).Bytes()
	switch err {
	default:
		return nil, errors.Wrap(err, "unexpected error from redis when retrieving")
	case redis.Nil:
		return nil, app.ErrNotFound
	case nil:
		return ret, nil
	}
}

func (b *RedisBackend) Delete(c context.Context, identifier string) error {
	return b.cli.Del(c, identifier).Err()
}
