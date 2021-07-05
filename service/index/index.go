package index

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/zllovesuki/b/box"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Options struct {
	Logger *zap.Logger
	Asset  box.AssetExtractor
}

type Service struct {
	Options
	indexPath string
}

func NewService(option Options) (*Service, error) {
	if err := option.validate(); err != nil {
		return nil, err
	}
	indexPath := option.Asset.Get("/index.html")
	if indexPath == "" {
		return nil, errors.New("unable to extract index.html")
	}
	return &Service{
		Options:   option,
		indexPath: indexPath,
	}, nil
}

func (o *Options) validate() error {
	if o.Logger == nil {
		return errors.New("missing logger")
	}
	if o.Asset == nil {
		return errors.New("missing asset extractor")
	}
	return nil
}

func (s *Service) index(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(s.indexPath)
	if err != nil {
		s.Logger.Error("unable to open index.html", zap.String("path", s.indexPath), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unexpected error")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "text/html")
	io.Copy(w, file)
}

func (s *Service) Route() http.Handler {
	r := chi.NewRouter()

	r.Get("/", s.index)

	return r
}
