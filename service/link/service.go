package link

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/validator"

	"github.com/go-chi/chi"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	prefix = "l-"
)

type Options struct {
	BaseURL string
	Backend app.Backend
	Logger  *zap.Logger
}

type Service struct {
	Options
}

func (o *Options) validate() error {
	if o.BaseURL == "" {
		return errors.New("baseurl cannot be empty")
	}
	if o.Backend == nil {
		return errors.New("missing backend")
	}
	if o.Logger == nil {
		return errors.New("missing logger")
	}
	return nil
}

func NewService(option Options) (*Service, error) {
	if err := option.validate(); err != nil {
		return nil, err
	}
	return &Service{
		Options: option,
	}, nil
}

type SaveLinkReq struct {
	URL string `json:"url"`
}

func (s *Service) saveLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req SaveLinkReq
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.WriteError(w, r, response.ErrInvalidJson())
		return
	}

	if !validator.URL(req.URL) {
		response.WriteError(w, r, response.
			ErrBadRequest().
			AddMessages("Provided URL is not valid"))
		return
	}

	err = s.Backend.Save(r.Context(), prefix+id, []byte(req.URL))
	switch err {
	default:
		s.Logger.Error("unable to save to backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
	case app.ErrConflict:
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
	case nil:
		response.WriteResponse(w, r, fmt.Sprintf("%s/%s", s.BaseURL, id))
	}
}

func (s *Service) retrieveLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	long, err := s.Backend.Retrieve(r.Context(), prefix+id)
	switch err {
	default:
		s.Logger.Error("unable to retrieve from backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected())
	case app.ErrNotFound:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "link not found")
	case nil:
		http.Redirect(w, r, string(long), http.StatusFound)
	}
}

// Route returns a mountable route for URL service
func (s *Service) Route() http.Handler {
	r := chi.NewRouter()

	r.Post("/{id}", s.saveLink)
	r.Get("/{id}", s.retrieveLink)

	return r
}
