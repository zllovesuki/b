package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zllovesuki/b/box"
	"github.com/zllovesuki/b/service"
	"github.com/zllovesuki/b/service/file"
	"github.com/zllovesuki/b/service/index"
	"github.com/zllovesuki/b/service/link"
	"github.com/zllovesuki/b/service/text"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

var configPath = flag.String("config", "config.yaml", "path to config.yaml")

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("unable to get logger: %v", err)
	}

	asset := box.GetAssetExtractor()
	defer asset.Close()

	dep, err := getConfig(logger, *configPath)
	if err != nil {
		logger.Fatal("getting configured dependencies", zap.Error(err))
	}

	index, err := index.NewService(index.Options{
		Logger: logger,
		Asset:  asset,
	})
	if err != nil {
		logger.Fatal("unable to get index service", zap.Error(err))
	}

	l, err := link.NewService(link.Options{
		BaseURL: dep.BaseURL,
		Backend: dep.LinkServiceBackend,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatal("unable to get link service", zap.Error(err))
	}

	t, err := text.NewService(text.Options{
		BaseURL: dep.BaseURL,
		Asset:   asset,
		Backend: dep.TextServiceBackend,
		Logger:  logger,
	})
	if err != nil {
		logger.Fatal("unable to get text service", zap.Error(err))
	}

	f, err := file.NewService(file.Options{
		BaseURL:         dep.BaseURL,
		MetadataBackend: dep.FileServiceMetadataBackend,
		FileBackend:     dep.FileServiceFastBackend,
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

	postGroup := r.Group(nil)
	postGroup.Use(middleware.NoCache)
	f.SaveRoute(postGroup)
	l.SaveRoute(postGroup)
	t.SaveRoute(postGroup)

	f.RetrieveRoute(r)
	l.RetrieveRoute(r)
	t.RetrieveRoute(r)

	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", dep.Port),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal("failed to listen for connection", zap.Error(err))
		}
	}()

	sugar := logger.Sugar()

	sugar.Infof("listening for connection on port %s", dep.Port)
	<-sigs
	sugar.Info("stopping server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("failed to shutdown gracefully", zap.Error(err))
	}

	sugar.Info("exited gracefully")
}
