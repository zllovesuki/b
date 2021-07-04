package text

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/response"
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

func TestGetText(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"
		ret := []byte("hello world!")

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return([]byte(ret), nil)

		dep.service.Route().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		buf, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, ret, buf)
	})

	t.Run("not found", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(nil, app.ErrNotFound)

		dep.service.Route().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("internal error", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"

		r, err := http.NewRequest("GET", "/"+id, nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), id).
			Return(nil, fmt.Errorf("error"))

		dep.service.Route().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestSaveText(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveTextReq{
			Text: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		}
		id := "wqrewr"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", "/"+id, bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Save(gomock.Any(), id, []byte(req.Text)).
			Return(nil)

		dep.service.Route().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var ret response.V1Response
		err = json.NewDecoder(resp.Body).Decode(&ret)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%s/%s", dep.baseURL, id), ret.Result)
	})

	t.Run("conflicting id should return conflict", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveTextReq{
			Text: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		}
		id := "hello"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", "/"+id, bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Save(gomock.Any(), id, []byte(req.Text)).
			Return(app.ErrConflict)

		dep.service.Route().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("internal error should return 500", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		req := SaveTextReq{
			Text: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		}
		id := "hello"

		body, err := json.Marshal(req)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", "/"+id, bytes.NewBuffer(body))
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Save(gomock.Any(), id, []byte(req.Text)).
			Return(fmt.Errorf("error"))

		dep.service.Route().ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}
