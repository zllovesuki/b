package text

import (
	"io/ioutil"
	"net/http"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"

	"github.com/buger/jsonparser"
	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	prefix = "t-"
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

// for reference
type SaveTextReq struct {
	Text string `json:"text"`
}

func (s *Service) saveText(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// TODO(zllovesuki): Consider using FastBackend
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.Logger.Error("unable to buffer request json", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	ret, err := jsonparser.GetString(buf, "text")
	if err != nil {
		response.WriteError(w, r, response.ErrInvalidJson())
		return
	}

	err = s.Backend.Save(r.Context(), prefix+id, []byte(ret))
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

	// TODO(zllovesuki): Consider using FastBackend
	text, err := s.Backend.Retrieve(r.Context(), prefix+id)
	if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrExpired) {
		response.WriteError(w, r, response.ErrNotFound().AddMessages("Text paste either expired or not found"))
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve text paste"))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(text)
}

// SaveRoute returns a mountable router for saving text paste.
// Alternatively, it can mount directly to the provided router.
func (s *Service) SaveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

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
