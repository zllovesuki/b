package index

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi"
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
}

func NewService(option Options) (*Service, error) {
	if err := option.validate(); err != nil {
		return nil, err
	}
	return &Service{
		Options: option,
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
	p := s.Asset.Get("/index.html")
	if p == "" {
		s.Logger.Error("unable to obtain index.html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unexpected error")
		return
	}

	file, err := os.Open(p)
	if err != nil {
		s.Logger.Error("unable to open index.html", zap.String("path", p), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unexpected error")
		return
	}

	io.Copy(w, file)
}

func (s *Service) Route() http.Handler {
	r := chi.NewRouter()

	r.Get("/", s.index)

	return r
}
