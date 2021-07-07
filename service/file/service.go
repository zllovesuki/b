package file

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	filePrefix = "f-"
	metaPrefix = "fm-"
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
	Version     int64
	Filename    string
	ContentType string
	Size        string
}

func (s *Service) retrieveFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	m, err := s.MetadataBackend.Retrieve(r.Context(), metaPrefix+id)
	if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrExpired) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "file not found")
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from metadata backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve file metadata"))
		return
	}

	var meta Metadata
	err = json.Unmarshal(m, &meta)
	if err != nil {
		s.Logger.Error("unable to decode file metadata", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Invalid file metadata"))
		return
	}

	fileReader, err := s.FileBackend.Retrieve(r.Context(), filePrefix+id)
	if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrExpired) {
		s.Logger.Error("file backend returned not found when metadata exists", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Failed to locate file via metadata"))
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from file backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve file"))
		return
	}

	defer fileReader.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", meta.Filename))
	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", meta.Size)
	io.Copy(w, app.NewCtxReader(r.Context(), fileReader))
}

func (s *Service) saveFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	form, err := r.MultipartReader()
	if err != nil {
		response.WriteError(w, r, response.ErrBadRequest().AddMessages("request is not multipart"))
		return
	}

	_, err = s.MetadataBackend.Retrieve(r.Context(), metaPrefix+id)
	if err == nil {
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
		return
	} else if errors.Is(err, app.ErrNotFound) || errors.Is(err, app.ErrExpired) {
		// fallthrough, allow override on expired file
	} else {
		s.Logger.Error("unable to check metadata backend prior to processing", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file"))
		return
	}

	p, err := form.NextPart()
	if err != nil && err != io.EOF {
		s.Logger.Error("unable to read next part from multipart reader", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	if p.FormName() != "file" {
		response.WriteError(w, r, response.ErrBadRequest().AddMessages("expecting \"file\" field"))
		return
	}

	file := bufio.NewReader(p)
	sniff, err := file.Peek(512)
	if err != nil {
		response.WriteError(w, r, response.ErrBadRequest().AddMessages("invalid file found"))
		return
	}
	contentType := http.DetectContentType(sniff)

	written, err := s.FileBackend.Save(r.Context(), filePrefix+id, io.NopCloser(app.NewCtxReader(r.Context(), file)))
	if errors.Is(err, app.ErrConflict) {
		s.Logger.Error("metadata backend reported no conflict when checking but reported conflict on save", zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file"))
		return
	} else if err != nil {
		s.Logger.Error("unable to save to file backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file"))
		return
	}

	meta := Metadata{
		Version:     1,
		Filename:    p.FileName(),
		ContentType: contentType,
		Size:        fmt.Sprint(written),
	}

	buf, err := json.Marshal(meta)
	if err != nil {
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	err = s.MetadataBackend.Save(r.Context(), metaPrefix+id, buf)
	if errors.Is(err, app.ErrConflict) {
		s.Logger.Error("conflicting identifier in metadata backend when file backend reports no conflict", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	} else if err != nil {
		s.Logger.Error("unable to save to metadata backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file metadata"))
		return
	}

	response.WriteResponse(w, r, service.Ret(s.BaseURL, filePrefix, id))
}

// SaveRoute returns a mountable router for saving file.
// Alternatively, it can mount directly to the provided router
func (s *Service) SaveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Post(service.Prefix(filePrefix, "{id:[a-zA-Z0-9]+}"), s.saveFile)

	return r
}

// RetrieveRoute returns a mountable router for retrieving files.
// Alternatively, it can mount directly to the provided router.
func (s *Service) RetrieveRoute(r chi.Router) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Get(service.Prefix(filePrefix, "{id:[a-zA-Z0-9]+}"), s.retrieveFile)

	return r
}
