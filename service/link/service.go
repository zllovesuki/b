package link

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"
	"github.com/zllovesuki/b/validator"

	"github.com/go-chi/chi/v5"
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
	if errors.Is(err, app.ErrConflict) {
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
		return
	} else if err != nil {
		s.Logger.Error("unable to save to backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save link"))
		return
	}

	response.WriteResponse(w, r, service.Ret(s.BaseURL, prefix, id))
}

func (s *Service) retrieveLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	long, err := s.Backend.Retrieve(r.Context(), prefix+id)
	if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrExpired) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "link not found")
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve link"))
		return
	}

	http.Redirect(w, r, string(long), http.StatusFound)
}

// SaveRoute returns a mountable router for saving url redirect
// Alternatively, it can mount directly to the provided router.
func (s *Service) SaveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Post(service.Prefix(prefix, "{id:[a-zA-Z0-9]+}"), s.saveLink)

	return r
}

// RetrieveRoute returns a mountable router for retrieving url redirect
// Alternatively, it can mount directly to the provided router.
func (s *Service) RetrieveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Get(service.Prefix(prefix, "{id:[a-zA-Z0-9]+}"), s.retrieveLink)

	return r
}
