package file

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

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
	switch err {
	default:
		s.Logger.Error("unable to retrieve from metadata backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve file metadata"))
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
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Invalid file metadata"))
		return
	}

	fileReader, err := s.FileBackend.Retrieve(r.Context(), filePrefix+id)
	switch err {
	default:
		s.Logger.Error("unable to retrieve from file backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to retrieve file"))
	case app.ErrNotFound:
		s.Logger.Error("file backend returned not found when metadata exists", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Failed to locate file via metadata"))
	case nil:
		defer fileReader.Close()
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", meta.Filename))
		w.Header().Set("Content-Type", meta.ContentType)
		w.Header().Set("Content-Length", meta.Size)
		io.Copy(w, fileReader)
	}
}

func (s *Service) saveFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	form, err := r.MultipartReader()
	if err != nil {
		s.Logger.Error("unable to open a multipart reader", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
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

	tmp, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("b-fast-%s-%s-tmp", filePrefix, id))
	if err != nil {
		s.Logger.Error("unable to open temporary file for buffering", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	length, err := io.Copy(tmp, file)
	if err != nil && err != io.EOF {
		s.Logger.Error("unable to write to temporary file", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		s.Logger.Error("unable to seek back to 0", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	meta := Metadata{
		Version:     1,
		Filename:    p.FileName(),
		ContentType: http.DetectContentType(sniff),
		Size:        fmt.Sprint(length),
	}

	buf, err := json.Marshal(meta)
	if err != nil {
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	err = s.MetadataBackend.Save(r.Context(), metaPrefix+id, buf)
	switch err {
	default:
		s.Logger.Error("unable to save to metadata backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file metadata"))
		return
	case app.ErrConflict:
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
		return
	case nil:
	}

	writer, err := s.FileBackend.Save(r.Context(), filePrefix+id)
	switch err {
	default:
		s.Logger.Error("unable to save to file backend", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file"))
	case app.ErrConflict:
		s.Logger.Error("conflicting identifier in file backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected())
	case nil:
		defer writer.Close()
		io.Copy(writer, tmp)
		response.WriteResponse(w, r, service.Ret(s.BaseURL, filePrefix, id))
	}
}

// Route returns a mountable route for file service
func (s *Service) Route(r *chi.Mux) http.Handler {
	if r == nil {
		r = chi.NewRouter()
	}

	r.Post(service.Prefix(filePrefix, "{id:[a-zA-Z0-9]+}"), s.saveFile)
	r.Get(service.Prefix(filePrefix, "{id:[a-zA-Z0-9]+}"), s.retrieveFile)

	return r
}
