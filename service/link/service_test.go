package link

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testDependencies struct {
	baseURL     string
	mockBackend *app.MockBackend
	recorder    *httptest.ResponseRecorder
	service     *Service
}

func getFixtures(t *testing.T) (*testDependencies, func()) {
	ctrl := gomock.NewController(t)
	mockBackend := app.NewMockBackend(ctrl)

	recorder := httptest.NewRecorder()

	logger := zaptest.NewLogger(t)

	base := "http://hello"

	service, err := NewService(Options{
		BaseURL: base,
		Backend: mockBackend,
		Logger:  logger,
	})
	require.NoError(t, err)

	return &testDependencies{
			baseURL:     base,
			mockBackend: mockBackend,
			recorder:    recorder,
			service:     service,
		}, func() {
			ctrl.Finish()
		}
}

func TestGetLink(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"
		ret := "https://google.com"

		r, err := http.NewRequest("GET", service.Prefix(prefix, id), nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), prefix+id).
			Return([]byte(ret), nil)

		dep.service.RetrieveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusFound, resp.StatusCode)
		require.Equal(t, ret, resp.Header.Get("Location"))
	})

	t.Run("not found", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", service.Prefix(prefix, id), nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), prefix+id).
			Return(nil, app.ErrNotFound)

		dep.service.RetrieveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("internal error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", service.Prefix(prefix, id), nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), prefix+id).
			Return(nil, fmt.Errorf("error"))

		dep.service.RetrieveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestSaveLink(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveLinkReq{
			URL: "https://google.com",
		}
		id := "wqrewr"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("PUT", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, []byte(req.URL), time.Duration(0)).
			Return(nil)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var ret response.V1Response
		err = json.NewDecoder(resp.Body).Decode(&ret)
		require.NoError(t, err)
		require.Equal(t, service.Ret(dep.baseURL, prefix, id), ret.Result)
	})

	t.Run("bad url should return bad request", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveLinkReq{
			URL: "hroiewhtoweqh",
		}
		id := "hello"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("PUT", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("conflicting id should return conflict", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveLinkReq{
			URL: "https://google.com",
		}
		id := "hello"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("PUT", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, []byte(req.URL), time.Duration(0)).
			Return(app.ErrConflict)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("internal error should return 500", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveLinkReq{
			URL: "https://google.com",
		}
		id := "hello"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("PUT", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, []byte(req.URL), time.Duration(0)).
			Return(fmt.Errorf("error"))

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("ttl request should be validated at router level", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		r, err := http.NewRequest("PUT", service.Prefix(prefix, "id/aewrw"), nil)
		require.NoError(t, err)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("ttl request should work", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		ttl := 60
		id := "hello"
		req := SaveLinkReq{
			URL: "https://google.com",
		}

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("PUT", service.Prefix(prefix, fmt.Sprintf("%s/%d", id, ttl)), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, []byte(req.URL), time.Second*time.Duration(ttl)).
			Return(nil)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("should reject large client payload", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		length := 4096
		payload := make([]byte, length)
		read, err := io.ReadFull(rand.Reader, payload)
		require.NoError(t, err)
		require.Equal(t, length, read)

		id := "hello"

		r, err := http.NewRequest("PUT", service.Prefix(prefix, id), bytes.NewBuffer(payload))
		require.NoError(t, err)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
