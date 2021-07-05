package link

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

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

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

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

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

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

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Save(gomock.Any(), prefix+id, []byte(req.URL)).
			Return(nil)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

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

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

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

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Save(gomock.Any(), prefix+id, []byte(req.URL)).
			Return(app.ErrConflict)

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

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

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Save(gomock.Any(), prefix+id, []byte(req.URL)).
			Return(fmt.Errorf("error"))

		dep.service.Route(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
