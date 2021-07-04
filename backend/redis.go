package backend

import (
	"context"
	"time"

	"github.com/zllovesuki/b/app"

	redis "github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

type basicRedis struct {
	cli *redis.Client
}

var _ app.Backend = &basicRedis{}

// NewBasicRedisBackend returns a redis backed storage for the application
func NewBasicRedisBackend(url string) (app.Backend, error) {
	b := &basicRedis{
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

func (b *basicRedis) Save(c context.Context, identifier string, data []byte) error {
	return b.SaveTTL(c, identifier, data, 0)
}

func (b *basicRedis) SaveTTL(c context.Context, identifier string, data []byte, ttl time.Duration) error {
	s, err := b.cli.SetNX(c, identifier, data, ttl).Result()
	if err != nil {
		return errors.Wrap(err, "unexpected error from redis when saving")
	}
	if !s {
		return app.ErrConflict
	}
	return nil
}

func (b *basicRedis) Retrieve(c context.Context, identifier string) ([]byte, error) {
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
