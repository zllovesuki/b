package main

import (
	"fmt"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/backend"
	"github.com/zllovesuki/b/fast"
	"github.com/zllovesuki/b/validator"
	"go.uber.org/zap"

	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/yaml"
	"github.com/pkg/errors"
)

var (
	availableBackends     = []string{"redis", "sqlite"}
	availableFastBackends = []string{"file", "s3"}
)

type dependencies struct {
	FileServiceMetadataBackend app.RemovableBackend
	FileServiceFastBackend     app.RemovableFastBackend
	LinkServiceBackend         app.Backend
	TextServiceBackend         app.FastBackend
	BaseURL                    string
	Port                       string
	Close                      func()
}

func verifyAtLeastOne(cfg *config.Config) error {
	hasBackend := false
	hasFastBackend := false
	for _, name := range availableBackends {
		hasBackend = hasBackend || cfg.Bool(fmt.Sprintf("backend.%s.enabled", name), false)
	}
	if !hasBackend {
		return errors.New("please enable at least one backend")
	}
	for _, name := range availableFastBackends {
		hasFastBackend = hasFastBackend || cfg.Bool(fmt.Sprintf("fastbackend.%s.enabled", name), false)
	}
	if !hasFastBackend {
		return errors.New("please enable at least one fastbackend")
	}
	return nil
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func verifyBackendConfigured(fm, f, l, t string) error {
	if !contains(availableBackends, fm) {
		return errors.New("please configure a valid metadata backend for file service")
	}
	if !contains(availableFastBackends, f) {
		return errors.New("please configure a valid file backend for file service")
	}
	if !contains(availableBackends, l) {
		return errors.New("please configure a valid backend for link service")
	}
	if !contains(availableFastBackends, t) {
		return errors.New("please configure a valid backend for text service")
	}
	return nil
}

func closer(logger *zap.Logger, f []func() error) func() {
	return func() {
		logger.Info("closing backends")
		for _, fn := range f {
			if err := fn(); err != nil {
				logger.Error("backend returns error on closing", zap.Error(err))
			}
		}
	}
}

func getConfig(logger *zap.Logger, configPath string) (*dependencies, error) {
	var err error

	cfg := config.New("b")
	cfg.AddDriver(yaml.Driver)

	err = cfg.LoadExists(configPath)
	if err != nil {
		return nil, err
	}

	if err := verifyAtLeastOne(cfg); err != nil {
		return nil, err
	}

	fm := cfg.String("service.file.metadata_backend")
	f := cfg.String("service.file.file_backend")
	l := cfg.String("service.link.backend")
	t := cfg.String("service.text.backend")

	if err := verifyBackendConfigured(fm, f, l, t); err != nil {
		return nil, err
	}

	baseURL := cfg.String("service.baseURL")
	if !validator.URL(baseURL) {
		return nil, errors.New("baseURL must be a valid URL (e.g. http://127.0.0.1:3000)")
	}

	port := cfg.String("service.port")
	if port == "" {
		return nil, errors.New("please specify a service port")
	}

	backendMap := map[string]app.RemovableBackend{}
	fastBackendMap := map[string]app.RemovableFastBackend{}
	closeFns := []func() error{}

	for _, name := range availableFastBackends {
		var f app.RemovableFastBackend
		enabled := cfg.Bool(fmt.Sprintf("fastbackend.%s.enabled", name), false)
		switch name {
		case "file":
			if !enabled {
				continue
			}
			dataPath := cfg.String("fastbackend.file.path")
			f, err = fast.NewFileFastBackend(dataPath)
			if err != nil {
				return nil, err
			}
		case "s3":
			if !enabled {
				continue
			}
			var s3Config fast.S3Config
			if err := cfg.MapStruct("fastbackend.s3", &s3Config); err != nil {
				return nil, errors.Wrap(err, "parsing s3 config")
			}
			f, err = fast.NewS3FastBackend(s3Config)
			if err != nil {
				return nil, err
			}
		}
		if f == nil {
			continue
		}
		fastBackendMap[name] = f
		closeFns = append(closeFns, f.Close)
	}

	for _, name := range availableBackends {
		var b app.RemovableBackend
		enabled := cfg.Bool(fmt.Sprintf("backend.%s.enabled", name), false)
		switch name {
		case "redis":
			if !enabled {
				continue
			}
			addr := cfg.String("backend.redis.addr")
			b, err = backend.NewRedisBackend(addr)
			if err != nil {
				return nil, err
			}
		case "sqlite":
			if !enabled {
				continue
			}
			path := cfg.String("backend.sqlite.path")
			b, err = backend.NewSQLiteBackend(path)
			if err != nil {
				return nil, err
			}
		}
		if b == nil {
			continue
		}
		backendMap[name] = b
		closeFns = append(closeFns, b.Close)
	}

	if backendMap[fm] == nil {
		return nil, errors.New("metadata backend not configured for file service")
	}
	if fastBackendMap[f] == nil {
		return nil, errors.New("file backend not configured for file service")
	}
	if backendMap[l] == nil {
		return nil, errors.New("backend not configured for link service")
	}
	if fastBackendMap[t] == nil {
		return nil, errors.New("backend not configured for text service")
	}

	log := logger.Sugar()
	log.Infof("metadata backend for file service configured with %T", backendMap[fm])
	log.Infof("file backend for file service configured with %T", fastBackendMap[f])
	log.Infof("backend for link service configured with %T", backendMap[l])
	log.Infof("backend for text service configured with %T", fastBackendMap[t])

	return &dependencies{
		Port:                       port,
		BaseURL:                    baseURL,
		FileServiceMetadataBackend: backendMap[fm],
		FileServiceFastBackend:     fastBackendMap[f],
		LinkServiceBackend:         backendMap[l],
		TextServiceBackend:         fastBackendMap[t],
		Close:                      closer(logger, closeFns),
	}, nil
}
