package file

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

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
	MetadataBackend app.RemovableBackend
	FileBackend     app.RemovableFastBackend
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
	if errors.Is(err, app.ErrNotFound) {
		response.WriteError(w, r, response.ErrNotFound().AddMessages("File either expired or does not exist"))
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from metadata backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Failed to locate file via metadata backend"))
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
	if errors.Is(err, app.ErrNotFound) {
		s.Logger.Error("file backend returned not found when metadata exists", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Failed to locate file via metadata backend"))
		return
	} else if err != nil {
		s.Logger.Error("unable to retrieve from file backend", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Failed to locate file via metadata backend"))
		return
	}

	defer fileReader.Close()
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": meta.Filename}))
	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", meta.Size)
	// TODO(zllovesuki): This fails on macOS with Firefox (server has closed the connection)
	written, err := io.Copy(w, app.NewCtxReader(r.Context(), fileReader))
	if err != nil {
		s.Logger.Warn("piping file buffer", zap.Error(err), zap.Int64("bytes-written", written))
	}
}

func (s *Service) saveFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var err error

	var form *multipart.Reader
	form, err = r.MultipartReader()
	if err != nil {
		response.WriteError(w, r, response.ErrBadRequest().AddMessages("request is not multipart"))
		return
	}

	_, err = s.MetadataBackend.Retrieve(r.Context(), metaPrefix+id)
	if err == nil {
		response.WriteError(w, r, response.ErrConflict().AddMessages("Conflicting identifier"))
		return
	} else if errors.Is(err, app.ErrNotFound) {
		// fallthrough, allow override on expired file
	} else {
		s.Logger.Error("unable to check metadata backend prior to processing", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file"))
		return
	}

	var p *multipart.Part
	p, err = form.NextPart()
	if err != nil && err != io.EOF {
		s.Logger.Error("unable to read next part from multipart reader", zap.Error(err))
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	if p.FormName() != "file" {
		response.WriteError(w, r, response.ErrBadRequest().AddMessages("expecting \"file\" field"))
		return
	}

	var buf []byte
	file := bufio.NewReader(p)
	buf, err = file.Peek(512)
	if err != nil {
		response.WriteError(w, r, response.ErrBadRequest().AddMessages("invalid file found"))
		return
	}
	contentType := http.DetectContentType(buf)

	// we will check if we encoutered any error during upload path and clean up
	defer func() {
		if err == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := s.FileBackend.Delete(ctx, filePrefix+id); err != nil {
				s.Logger.Error("removing failed upload from file backend", zap.Error(err), zap.String("id", id))
			}
		}()
		go func() {
			defer wg.Done()
			if err := s.MetadataBackend.Delete(ctx, metaPrefix+id); err != nil {
				s.Logger.Error("removing failed upload from metadata backend", zap.Error(err), zap.String("id", id))
			}
		}()
		wg.Wait()
	}()

	var written int64
	written, err = s.FileBackend.SaveTTL(r.Context(), filePrefix+id, io.NopCloser(app.NewCtxReader(r.Context(), file)), 0)
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

	buf, err = json.Marshal(meta)
	if err != nil {
		response.WriteError(w, r, response.ErrUnexpected())
		return
	}

	err = s.MetadataBackend.SaveTTL(r.Context(), metaPrefix+id, buf, 0)
	if errors.Is(err, app.ErrConflict) {
		s.Logger.Error("conflicting identifier in metadata backend when previous lookup reports no conflict", zap.Error(err), zap.String("id", id))
		response.WriteError(w, r, response.ErrUnexpected().AddMessages("Unable to save file metadata"))
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

	r.Put(service.Prefix(filePrefix, "{id:[a-zA-Z0-9]+}"), s.saveFile)

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
