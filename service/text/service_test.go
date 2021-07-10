package text

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zllovesuki/b/app"
	"github.com/zllovesuki/b/box"
	"github.com/zllovesuki/b/response"
	"github.com/zllovesuki/b/service"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testDependencies struct {
	baseURL     string
	mockBackend *app.MockFastBackend
	recorder    *httptest.ResponseRecorder
	service     *Service
}

var asset = box.GetAssetExtractor()

func getFixtures(t *testing.T) (*testDependencies, func()) {
	ctrl := gomock.NewController(t)
	mockBackend := app.NewMockFastBackend(ctrl)

	recorder := httptest.NewRecorder()

	logger := zaptest.NewLogger(t)

	base := "http://hello"

	service, err := NewService(Options{
		BaseURL: base,
		Asset:   asset,
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
		txt := []byte("hello world")
		ret := bytes.NewBuffer(txt)

		r, err := http.NewRequest("GET", service.Prefix(prefix, id), nil)
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), prefix+id).
			Return(io.NopCloser(ret), nil)

		dep.service.RetrieveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		buf, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, txt, buf)
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

	t.Run("naughty id", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "../../../etc/hello"

		r, err := http.NewRequest("GET", service.Prefix(prefix, id), nil)
		require.NoError(t, err)

		dep.service.RetrieveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run(".html in url should return highlight.js page", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "hello"
		txt := []byte("hello world")
		ret := bytes.NewBuffer(txt)
		uri := service.Prefix(prefix, fmt.Sprintf("%s.html", id))

		r, err := http.NewRequest("GET", uri, nil)
		r.RequestURI = uri // monkey patch
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			Retrieve(gomock.Any(), prefix+id).
			Return(io.NopCloser(ret), nil)

		dep.service.RetrieveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

		buf, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(buf), string(txt))
	})
}

func TestSaveText(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		txt := []byte("hello world")
		body := bytes.NewReader(txt)
		id := "wqrewr"

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, r.Body, time.Duration(0)).
			Return(int64(len(txt)), nil)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var ret response.V1Response
		err = json.NewDecoder(resp.Body).Decode(&ret)
		require.NoError(t, err)
		require.Equal(t, service.Ret(dep.baseURL, prefix, id), ret.Result)
	})

	t.Run("conflicting id should return conflict", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		txt := []byte("hello world")
		body := bytes.NewReader(txt)
		id := "wqrewr"
		r, err := http.NewRequest("POST", service.Prefix(prefix, id), body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, r.Body, time.Duration(0)).
			Return(int64(0), app.ErrConflict)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})

	t.Run("internal error should return 500", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		txt := []byte("hello world")
		body := bytes.NewReader(txt)
		id := "wqrewr"
		r, err := http.NewRequest("POST", service.Prefix(prefix, id), body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, r.Body, time.Duration(0)).
			Return(int64(0), fmt.Errorf("error"))

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()
		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("naughty id", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "../../../etc/hello"

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), nil)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, err)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)

	})

	t.Run("should reject non x-www-form-urlencoded request", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		id := "wqrewr"
		thing := struct {
			Text string
		}{
			Text: "hi",
		}
		buf, err := json.Marshal(&thing)
		require.NoError(t, err)

		r, err := http.NewRequest("POST", service.Prefix(prefix, id), bytes.NewBuffer(buf))
		r.Header.Add("Content-Type", "application/json")
		require.NoError(t, err)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("ttl request should be validated at router level", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		r, err := http.NewRequest("POST", service.Prefix(prefix, "id/abce"), nil)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, err)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("ttl request should work", func(t *testing.T) {
		dep, finish := getFixtures(t)
		defer finish()

		ttl := 60
		txt := []byte("hello world")
		body := bytes.NewReader(txt)
		id := "wqrewr"
		r, err := http.NewRequest("POST", service.Prefix(prefix, fmt.Sprintf("%s/%d", id, ttl)), body)
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		require.NoError(t, err)

		dep.mockBackend.EXPECT().
			SaveTTL(gomock.Any(), prefix+id, r.Body, time.Second*time.Duration(ttl)).
			Return(int64(len(txt)), nil)

		dep.service.SaveRoute(nil).ServeHTTP(dep.recorder, r)

		resp := dep.recorder.Result()

		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
