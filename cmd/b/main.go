package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/zllovesuki/b/backend"
	"github.com/zllovesuki/b/box"
	"github.com/zllovesuki/b/fast"
	"github.com/zllovesuki/b/service"
	"github.com/zllovesuki/b/service/file"
	"github.com/zllovesuki/b/service/index"
	"github.com/zllovesuki/b/service/link"
	"github.com/zllovesuki/b/service/text"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/docgen"
	"go.uber.org/zap"
)

var routes = flag.Bool("routes", false, "Generate router documentation")

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("unable to get logger: %v", err)
	}

	asset := box.GetAssetExtractor()
	defer asset.Close()

	redis, err := backend.NewRedisBackend("127.0.0.1:6379")
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
		BaseURL: "http://127.0.0.1:3000",
		Backend: redis,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatal("unable to get link service", zap.Error(err))
	}

	t, err := text.NewService(text.Options{
		BaseURL: "http://127.0.0.1:3000",
		Backend: redis,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatal("unable to get text service", zap.Error(err))
	}

	s, err := fast.NewFileFastBackend("data")
	if err != nil {
		logger.Fatal("unable to get file fast backend", zap.Error(err))
	}
	// s, err := fast.NewS3FastBackend(fast.S3Config{
	// 	Bucket:         "bfast",
	// 	Endpoint:       "127.0.0.1:9000",
	// 	Region:         "us-east-1",
	// 	AccessKey:      "minioadmin",
	// 	AccessSecret:   "minioadmin",
	// 	DisableSSL:     true,
	// 	ForcePathStyle: true,
	// })
	// if err != nil {
	// 	logger.Fatal("unable to get s3 fast backend", zap.Error(err))
	// }

	f, err := file.NewService(file.Options{
		BaseURL:         "http://127.0.0.1:3000",
		MetadataBackend: redis,
		FileBackend:     s,
		Logger:          logger,
	})
	if err != nil {
		logger.Fatal("unable to get file service", zap.Error(err))
	}

	r := chi.NewRouter()

	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(middleware.RequestID)
	r.Use(service.Recovery(logger))
	r.Mount("/debug", middleware.Profiler())

	r.Mount("/", index.Route())
	l.Route(r)
	t.Route(r)
	f.Route(r)

	if *routes {
		fmt.Println(docgen.JSONRoutesDoc(r))
		return
	}

	http.ListenAndServe(":3000", r)
}
