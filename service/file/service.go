package file

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"go.uber.org/zap"
)

type Options struct {
	BaseURL         string
	MetadataBackend app.Backend
	FileBackend     app.FastBackend
	Logger          *zap.Logger
}

type Service struct {
	Options
}

func (o *Options) validate() error {
	if o.BaseURL == "" {
		return errors.New("baseurl cannot be empty")
	}
	if o.MetadataBackend == nil {
		return errors.New("missing metadata backend")
	}
	if o.FileBackend == nil {
		return errors.New("missing file backend")
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

type Metadata struct {
	Filename    string
	ContentType string
}

func (s *Service) retrieveFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	m, err := s.MetadataBackend.Retrieve(r.Context(), id)
	switch err {
	default:
		s.Logger.Error("unable to retrieve from metadata backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	case app.ErrNotFound:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "file not found")
		return
	case nil:
	}

	var meta Metadata
	err = json.Unmarshal(m, &meta)
	if err != nil {
		s.Logger.Error("unable to decode file metadata", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	fileReader, err := s.FileBackend.Retrieve(r.Context(), id)
	switch err {
	default:
		s.Logger.Error("unable to retrieve from metadata backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected())
	case app.ErrNotFound:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "file not found")
	case nil:
		defer fileReader.Close()
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", meta.Filename))
		w.Header().Set("Content-Type", meta.ContentType)
		io.Copy(w, fileReader)
	}
}

func (s *Service) saveFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	r.ParseMultipartForm(100 << 20) // 100 MB
	file, header, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, r, response.ErrBadRequest())
		return
	}
	defer file.Close()

	fileHeader := make([]byte, 512)
	if _, err := file.Read(fileHeader); err != nil {
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}
	if _, err := file.Seek(0, 0); err != nil {
		response.WriteError(w, r, response.ErrBadRequest())
		return
	}

	meta := Metadata{
		Filename:    header.Filename,
		ContentType: http.DetectContentType(fileHeader),
	}

	buf, err := json.Marshal(meta)
	if err != nil {
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	err = s.MetadataBackend.Save(r.Context(), id, buf)
	switch err {
	default:
		s.Logger.Error("unable to save to metadata backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	case app.ErrConflict:
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
		return
	case nil:
	}

	writer, err := s.FileBackend.Save(r.Context(), id)
	switch err {
	default:
		s.Logger.Error("unable to save to file backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
	case app.ErrConflict:
		s.Logger.Error("conflicting identifier in file backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
	case nil:
		defer writer.Close()
		io.Copy(writer, file)
		response.WriteResponse(w, r, fmt.Sprintf("%s/%s", s.BaseURL, id))
	}
}

// SaveRoute returns a mountable route for saving file
func (s *Service) SaveRoute() http.Handler {
	r := chi.NewRouter()

	r.Post("/{id}", s.saveFile)

	return r
}

// RetrieveRoute returns a mountable route for retrieving uploaded file
func (s *Service) RetrieveRoute() http.Handler {
	r := chi.NewRouter()

	r.Get("/{id}", s.retrieveFile)

	return r
}
