package text

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	prefix = "t-"
)

type Options struct {
	BaseURL string
	Backend app.FastBackend
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

func (s *Service) saveText(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ttlStr := chi.URLParam(r, "ttl")
	var ttl int64
	if ttlStr != "" {
		// this should already be validated at router level (only numbers are allowed)
		ttl, _ = strconv.ParseInt(ttlStr, 10, 64)
	}

	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		response.WriteError(w, r, response.ErrBadRequest().
			AddMessages("Request content-type is not application/x-www-form-urlencoded").
			AddMessages("If you are using curl, please use the following command:").
			AddMessages("cat foo.txt | curl --data-binary @- http://example.com/t-foo"))
		return
	}

	_, err := s.Backend.SaveTTL(r.Context(), prefix+id, r.Body, time.Second*time.Duration(ttl))
	if errors.Is(err, app.ErrConflict) {
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
		return
	} else if err != nil {
		s.Logger.Error("unable to save to backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save text paste"))
		return
	}

	response.WriteResponse(w, r, service.Ret(s.BaseURL, prefix, id))
}

func (s *Service) retrieveText(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	text, err := s.Backend.Retrieve(r.Context(), prefix+id)
	if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrExpired) {
		response.WriteError(w, r, response.ErrNotFound().AddMessages("Text paste either expired or not found"))
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve text paste"))
		return
	}
	defer text.Close()

	w.Header().Set("Content-Type", "text/plain")
	io.Copy(w, text)
}

// SaveRoute returns a mountable router for saving text paste.
// Alternatively, it can mount directly to the provided router.
func (s *Service) SaveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Post(service.Prefix(prefix, "{id:[a-zA-Z0-9]+}/{ttl:[0-9]+}"), s.saveText)
	r.Post(service.Prefix(prefix, "{id:[a-zA-Z0-9]+}"), s.saveText)

	return r
}

// RetrieveRoute returns a mountable router for saving text paste.
// Alternatively, it can mount directly to the provided router.
func (s *Service) RetrieveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Get(service.Prefix(prefix, "{id:[a-zA-Z0-9]+}"), s.retrieveText)

	return r
}
