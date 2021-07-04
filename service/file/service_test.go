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

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"go.uber.org/zap"
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

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

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

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(buf, nil)

		dep.mockFileBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(dep.testFile, nil)

		dep.service.RetrieveRoute().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, meta.ContentType, resp.Header.Get("Content-Type"))
	})

	t.Run("not found", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(nil, app.ErrNotFound)

		dep.service.RetrieveRoute().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("metadata backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(nil, fmt.Errorf("error"))

		dep.service.RetrieveRoute().ServeHTTP(dep.recorder, r)

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

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockMetadataBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(buf, nil)

		dep.mockFileBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(nil, fmt.Errorf("error"))

		dep.service.RetrieveRoute().ServeHTTP(dep.recorder, r)

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
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		body, writer, length := getMultipart(t, dep.testFile, meta)

		r, err := http.NewRequest("POST", "/"+id, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		mockRecorder := &mockWriter{buf: make([]byte, 0)}

		dep.mockMetadataBackend.EXPECT().
			Save(gomock.Any(), id, buf).
			Return(nil)

		dep.mockFileBackend.EXPECT().
			Save(gomock.Any(), id).
			Return(mockRecorder, nil)

		dep.service.SaveRoute().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var ret response.V1Response
		err = json.NewDecoder(resp.Body).Decode(&ret)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s/%s", dep.baseURL, id), ret.Result)
		require.Equal(t, length, int64(len(mockRecorder.buf)))
	})

	t.Run("conflicting id should return conflict", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		body, writer, _ := getMultipart(t, dep.testFile, meta)

		r, err := http.NewRequest("POST", "/"+id, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Save(gomock.Any(), id, buf).
			Return(app.ErrConflict)

		dep.service.SaveRoute().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("metadata backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		body, writer, _ := getMultipart(t, dep.testFile, meta)

		r, err := http.NewRequest("POST", "/"+id, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Save(gomock.Any(), id, buf).
			Return(fmt.Errorf("error"))

		dep.service.SaveRoute().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("file backend error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		meta := Metadata{
			Filename:    "image.jpg",
			ContentType: "image/jpeg",
		}
		buf, err := json.Marshal(meta)
		require.NoError(t, err)

		body, writer, _ := getMultipart(t, dep.testFile, meta)

		r, err := http.NewRequest("POST", "/"+id, body)
		require.NoError(t, err)
		r.Header.Add("Content-Type", writer.FormDataContentType())

		dep.mockMetadataBackend.EXPECT().
			Save(gomock.Any(), id, buf).
			Return(nil)

		dep.mockFileBackend.EXPECT().
			Save(gomock.Any(), id).
			Return(nil, fmt.Errorf("error"))

		dep.service.SaveRoute().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}