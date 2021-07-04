package main

import (
	"log"
	"net/http"

	"github.com/zllovesuki/b/backend"
	"github.com/zllovesuki/b/box"
	"github.com/zllovesuki/b/service/index"
	"github.com/zllovesuki/b/service/link"
	"github.com/zllovesuki/b/service/text"

	"github.com/go-chi/chi"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("unable to get logger: %v", err)
	}

	asset := box.GetAssetExtractor()
	defer asset.Close()

	redis, err := backend.NewBasicRedisBackend("127.0.0.1:6379")
	if err != nil {
		logger.Fatal("unable to connect to redis", zap.Error(err))
	}

	index, err := index.NewService(index.Options{
		Logger: logger,
		Asset:  asset,
	})
	if err != nil {
		logger.Fatal("unable to get index service", zap.Error(err))
	}

	l, err := link.NewService(link.Options{
		BaseURL: "http://127.0.0.1:3000/l",
		Backend: redis,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatal("unable to get link service", zap.Error(err))
	}

	t, err := text.NewService(text.Options{
		BaseURL: "http://127.0.0.1:3000/t",
		Backend: redis,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatal("unable to get text service", zap.Error(err))
	}

	r := chi.NewRouter()
	r.Mount("/", index.Route())
	r.Mount("/l", l.Route())
	r.Mount("/t", t.Route())

	http.ListenAndServe(":3000", r)
}
