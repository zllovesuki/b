package text

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"

	"github.com/buger/jsonparser"
	"github.com/go-chi/chi"
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

	ret, t, _, err := jsonparser.Get(buf, "text")
	if err != nil || t != jsonparser.String {
		response.WriteError(w, r, response.ErrInvalidJson())
		return
	}

	err = s.Backend.Save(r.Context(), prefix+id, ret)
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

func (s *Service) retrieveText(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// TODO(zllovesuki): Consider using FastBackend
	text, err := s.Backend.Retrieve(r.Context(), prefix+id)
	switch err {
	default:
		s.Logger.Error("unable to retrieve from backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected())
	case app.ErrNotFound:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "text not found")
	case nil:
		w.Write(text)
	}
}

// Route returns a mountable route for text service
func (s *Service) Route() http.Handler {
	r := chi.NewRouter()

	r.Post("/{id}", s.saveText)
	r.Get("/{id}", s.retrieveText)

	return r
}
