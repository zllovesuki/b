package file

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// fixtures image obtained from https://unsplash.com/photos/cpNor3rFdWk
// (c) Marek Piwnicki https://unsplash.com/@marekpiwnicki

type testDependencies struct {
	baseURL             string
	mockMetadataBackend *app.MockBackend
	mockFileBackend     *app.MockFastBackend
	recorder            *httptest.ResponseRecorder
	service             *Service
	testFile            *os.File
}

func getFixtures(t *testing.T) (*testDependencies, func()) {
	ctrl := gomock.NewController(t)
	mockFileBackend := app.NewMockFastBackend(ctrl)
	mockMetadataBackend := app.NewMockBackend(ctrl)

	recorder := httptest.NewRecorder()

	logger := zaptest.NewLogger(t)

	base := "http://hello"

	service, err := NewService(Options{
		BaseURL:         base,
		MetadataBackend: mockMetadataBackend,
		FileBackend:     mockFileBackend,
		Logger:          logger,
	})
	require.NoError(t, err)

	file, err := os.Open(filepath.Join("fixtures", "image.jpg"))
	require.NoError(t, err)

	return &testDependencies{
			baseURL:             base,
			mockMetadataBackend: mockMetadataBackend,
			mockFileBackend:     mockFileBackend,
			recorder:            recorder,
			service:             service,
			testFile:            file,
		}, func() {
			file.Close()
			ctrl.Finish()
		}
}

func TestGetFile(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"
		meta := Metadata{
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		r, err := http.NewRequest("GET", service.Prefix(filePrefix, id), nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(buf, nil)

		dep.mockFileBackend.EXPECT().
			Retrieve(gomock.Any(), filePrefix+id).
			Return(dep.testFile, nil)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, meta.ContentType, resp.Header.Get("Content-Type"))
	})

	t.Run("not found", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", service.Prefix(filePrefix, id), nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(nil, app.ErrNotFound)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("metadata backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", service.Prefix(filePrefix, id), nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(nil, fmt.Errorf("error"))

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("file backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"
		meta := Metadata{
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		r, err := http.NewRequest("GET", service.Prefix(filePrefix, id), nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(buf, nil)

		dep.mockFileBackend.EXPECT().
			Retrieve(gomock.Any(), filePrefix+id).
			Return(nil, fmt.Errorf("error"))

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("file backend reported missing but metadata reported found", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		r, err := http.NewRequest("GET", service.Prefix(filePrefix, id), nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(buf, nil)

		dep.mockFileBackend.EXPECT().
			Retrieve(gomock.Any(), filePrefix+id).
			Return(nil, app.ErrNotFound)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("corrupted metadata should return err", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"

		r, err := http.NewRequest("GET", service.Prefix(filePrefix, id), nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return([]byte("hi"), nil)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	})
}

type mockWriter struct {
	buf []byte
}

var _ io.WriteCloser = &mockWriter{}

func (m *mockWriter) Write(b []byte) (int, error) {
	p := len(m.buf)
	m.buf = append(m.buf, b...)
	a := len(m.buf)
	return a - p, nil
}

func (m *mockWriter) Close() error {
	return nil
}

func getMultipart(t *testing.T, file *os.File, meta Metadata) (io.Reader, *multipart.Writer, int64) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	defer writer.Close()
	part, err := writer.CreateFormFile("file", meta.Filename)
	require.NoError(t, err)
	length, err := io.Copy(part, file)
	require.NoError(t, err)
	return body, writer, length
}

func TestSaveFile(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Version:     1,
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}

		body, writer, length := getMultipart(t, dep.testFile, meta)
		meta.Size = fmt.Sprint(length)
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", service.Prefix(filePrefix, id), body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(nil, app.ErrNotFound)

		dep.mockMetadataBackend.EXPECT().
			Save(gomock.Any(), metaPrefix+id, buf).
			Return(nil)

		dep.mockFileBackend.EXPECT().
			Save(gomock.Any(), filePrefix+id, gomock.Any()).
			Return(length, nil)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var ret response.V1Response
		err = json.NewDecoder(resp.Body).Decode(&ret)
		require.NoError(t, err)
		require.Equal(t, service.Ret(dep.baseURL, filePrefix, id), ret.Result)
	})

	t.Run("conflicting id should return conflict", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Version:     1,
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}

		body, writer, length := getMultipart(t, dep.testFile, meta)
		meta.Size = fmt.Sprint(length)
		buf, err := json.Marshal(&meta)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", service.Prefix(filePrefix, id), body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(buf, nil)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("metadata backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Version:     1,
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}

		body, writer, _ := getMultipart(t, dep.testFile, meta)

		r, err := http.NewRequest("POST", service.Prefix(filePrefix, id), body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(nil, fmt.Errorf("error"))

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("file backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Version:     1,
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}

		body, writer, _ := getMultipart(t, dep.testFile, meta)

		r, err := http.NewRequest("POST", service.Prefix(filePrefix, id), body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(nil, app.ErrNotFound)

		dep.mockFileBackend.EXPECT().
			Save(gomock.Any(), filePrefix+id, gomock.Any()).
			Return(int64(0), fmt.Errorf("error"))

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("conflict in meta backend when checking reported not found", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Version:     1,
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}

		body, writer, length := getMultipart(t, dep.testFile, meta)
		meta.Size = fmt.Sprint(length)
		buf, err := json.Marshal(&meta)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", service.Prefix(filePrefix, id), body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), metaPrefix+id).
			Return(nil, app.ErrNotFound)

		dep.mockFileBackend.EXPECT().
			Save(gomock.Any(), filePrefix+id, gomock.Any()).
			Return(length, nil)

		dep.mockMetadataBackend.EXPECT().
			Save(gomock.Any(), metaPrefix+id, buf).
			Return(app.ErrConflict)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
