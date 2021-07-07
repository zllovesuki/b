package fast

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/zllovesuki/b/app"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type S3Config struct {
	Bucket         string
	Endpoint       string
	Region         string
	DisableSSL     bool
	AccessKey      string
	AccessSecret   string
	ForcePathStyle bool
	Logger         *zap.Logger
}

func (s S3Config) validate() error {
	if s.Region == "" {
		return errors.New("region cannot be empty")
	}
	if s.Bucket == "" {
		return errors.New("bucket cannot be empty")
	}
	if s.AccessKey == "" {
		return errors.New("access key cannot be empty")
	}
	if s.AccessSecret == "" {
		return errors.New("access secret cannot be empty")
	}
	return nil
}

const (
	metaCreated = "B-Created-Date"
	metaTTL     = "B-Time-To-Live"
)

type S3FastBackend struct {
	config S3Config
	mc     *minio.Client
}

var _ app.FastBackend = &S3FastBackend{}
var _ app.Removable = &S3FastBackend{}

func NewS3FastBackend(conf S3Config) (*S3FastBackend, error) {
	if err := conf.validate(); err != nil {
		return nil, err
	}
	option := &minio.Options{
		Region:       conf.Region,
		Secure:       !conf.DisableSSL,
		Creds:        credentials.NewStaticV4(conf.AccessKey, conf.AccessSecret, ""),
		BucketLookup: minio.BucketLookupDNS,
	}
	if conf.ForcePathStyle {
		option.BucketLookup = minio.BucketLookupPath
	}
	mc, err := minio.New(conf.Endpoint, option)
	if err != nil {
		return nil, errors.Wrap(err, "creating s3 client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	found, err := mc.BucketExists(ctx, conf.Bucket)
	if err != nil {
		return nil, errors.Wrap(err, "checking bucket existence")
	}
	if !found {
		if err := mc.MakeBucket(ctx, conf.Bucket, minio.MakeBucketOptions{
			Region: conf.Region,
		}); err != nil {
			return nil, errors.Wrap(err, "creating bucket")
		}
	}

	return &S3FastBackend{
		config: conf,
		mc:     mc,
	}, nil
}

func (s *S3FastBackend) Save(c context.Context, identifier string, r io.ReadCloser) (int64, error) {
	return s.SaveTTL(c, identifier, r, 0)
}

func (s *S3FastBackend) SaveTTL(c context.Context, identifier string, r io.ReadCloser, ttl time.Duration) (int64, error) {
	defer r.Close()

	exist := true

	info, err := s.mc.StatObject(c, s.config.Bucket, identifier, minio.StatObjectOptions{})
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.StatusCode == http.StatusNotFound {
			exist = false
		} else {
			return 0, errors.Wrap(err, "stat object for checking existence")
		}
	}

	if exist {
		whenStr := info.UserMetadata[metaCreated]
		ttlStr := info.UserMetadata[metaTTL]
		when, err := time.Parse(time.RFC3339, whenStr)
		if err != nil {
			return 0, errors.Wrap(err, "parsing created date")
		}
		exp, err := time.ParseDuration(ttlStr)
		if err != nil {
			return 0, errors.Wrap(err, "parsing ttl")
		}
		if exp == 0 || time.Now().UTC().Before(when.UTC().Add(exp)) {
			return 0, app.ErrConflict
		}
	}

	u, err := s.mc.PutObject(c, s.config.Bucket, identifier, r, -1, minio.PutObjectOptions{
		PartSize: 16 << 20, // 16MiB
		UserMetadata: map[string]string{
			metaCreated: time.Now().UTC().Format(time.RFC3339),
			metaTTL:     ttl.String(),
		},
	})
	if err != nil {
		return 0, errors.Wrap(err, "uploading to s3")
	}

	return u.Size, nil
}

func (s *S3FastBackend) Retrieve(c context.Context, identifier string) (io.ReadCloser, error) {
	info, err := s.mc.StatObject(c, s.config.Bucket, identifier, minio.StatObjectOptions{})
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.StatusCode == http.StatusNotFound {
			return nil, app.ErrNotFound
		} else {
			return nil, errors.Wrap(err, "testing existence")
		}
	}

	whenStr := info.UserMetadata[metaCreated]
	ttlStr := info.UserMetadata[metaTTL]
	when, err := time.Parse(time.RFC3339, whenStr)
	if err != nil {
		return nil, errors.Wrap(err, "parsing created date")
	}
	exp, err := time.ParseDuration(ttlStr)
	if err != nil {
		return nil, errors.Wrap(err, "parsing ttl")
	}
	if exp != 0 && time.Now().UTC().After(when.UTC().Add(exp)) {
		return nil, app.ErrExpired
	}

	reader, err := s.mc.GetObject(c, s.config.Bucket, identifier, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting reader for file")
	}

	return reader, nil
}

func (s *S3FastBackend) Delete(c context.Context, identifier string) error {
	return s.mc.RemoveObject(c, s.config.Bucket, identifier, minio.RemoveObjectOptions{})
}
