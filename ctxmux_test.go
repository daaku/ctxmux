package ctxmux_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/daaku/ctxmux"
	"github.com/facebookgo/ensure"
	"github.com/julienschmidt/httprouter"
)

func TestContextWithNoParams(t *testing.T) {
	var nilParams httprouter.Params
	ensure.DeepEqual(t, ctxmux.ContextParams(context.Background()), nilParams)
}

func TestContextWithFromParams(t *testing.T) {
	p := httprouter.Params{}
	ctx := ctxmux.WithParams(context.Background(), p)
	actual := ctxmux.ContextParams(ctx)
	ensure.DeepEqual(t, actual, p)
}

func TestHTTPHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := &http.Request{}
	var actualW http.ResponseWriter
	var actualR *http.Request
	h := ctxmux.HTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualW = w
		actualR = r
	}))
	ensure.Nil(t, h(w, r))
	ensure.DeepEqual(t, actualW, w)
	ensure.DeepEqual(t, actualR, r)
}

func TestHTTPHandlerFunc(t *testing.T) {
	w := httptest.NewRecorder()
	r := &http.Request{}
	var actualW http.ResponseWriter
	var actualR *http.Request
	h := ctxmux.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualW = w
		actualR = r
	})
	ensure.Nil(t, h(w, r))
	ensure.DeepEqual(t, actualW, w)
	ensure.DeepEqual(t, actualR, r)
}

func TestNewError(t *testing.T) {
	givenErr := errors.New("")
	mux, err := ctxmux.New(
		func(*ctxmux.Mux) error {
			return givenErr
		},
	)
	ensure.True(t, mux == nil)
	ensure.DeepEqual(t, err, givenErr)
}

func TestWrapMethods(t *testing.T) {
	cases := []struct {
		Method   string
		Register func(*ctxmux.Mux, string, ctxmux.Handler)
	}{
		{Method: "HEAD", Register: (*ctxmux.Mux).HEAD},
		{Method: "GET", Register: (*ctxmux.Mux).GET},
		{Method: "POST", Register: (*ctxmux.Mux).POST},
		{Method: "PUT", Register: (*ctxmux.Mux).PUT},
		{Method: "DELETE", Register: (*ctxmux.Mux).DELETE},
		{Method: "PATCH", Register: (*ctxmux.Mux).PATCH},
	}
	const key = int(1)
	const val = int(2)
	body := []byte("body")
	for _, c := range cases {
		mux, err := ctxmux.New(
			ctxmux.MuxContextChanger(func(r *http.Request) (*http.Request, error) {
				return r.WithContext(context.WithValue(r.Context(), key, val)), nil
			}),
		)
		ensure.Nil(t, err)
		hw := httptest.NewRecorder()
		hr := &http.Request{
			Method: c.Method,
			URL: &url.URL{
				Path: "/",
			},
		}
		c.Register(mux, hr.URL.Path, func(w http.ResponseWriter, r *http.Request) error {
			ensure.DeepEqual(t, r.Context().Value(key), val)
			w.Write(body)
			return nil
		})
		mux.ServeHTTP(hw, hr)
		ensure.DeepEqual(t, hw.Body.Bytes(), body)
	}
}

func TestMuxContextMakerError(t *testing.T) {
	givenErr := errors.New("")
	var actualErr error
	mux, err := ctxmux.New(
		ctxmux.MuxContextChanger(func(r *http.Request) (*http.Request, error) {
			return nil, givenErr
		}),
		ctxmux.MuxErrorHandler(
			func(w http.ResponseWriter, r *http.Request, err error) {
				actualErr = err
			}),
	)
	ensure.Nil(t, err)
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.GET(hr.URL.Path, func(http.ResponseWriter, *http.Request) error {
		panic("not reached")
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, actualErr, givenErr)
}

func TestHandleCustomMethod(t *testing.T) {
	mux, err := ctxmux.New()
	ensure.Nil(t, err)
	const method = "FOO"
	body := []byte("body")
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: method,
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.Handler(method, hr.URL.Path, func(w http.ResponseWriter, r *http.Request) error {
		w.Write(body)
		return nil
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, hw.Body.Bytes(), body)
}

func TestHandlerReturnErr(t *testing.T) {
	givenErr := errors.New("")
	var actualErr error
	mux, err := ctxmux.New(
		ctxmux.MuxErrorHandler(
			func(w http.ResponseWriter, r *http.Request, err error) {
				actualErr = err
			}),
	)
	ensure.Nil(t, err)
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.GET(hr.URL.Path, func(http.ResponseWriter, *http.Request) error {
		return givenErr
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, actualErr, givenErr)
}

func TestHandlerPanic(t *testing.T) {
	var actualPanic interface{}
	mux, err := ctxmux.New(
		ctxmux.MuxPanicHandler(
			func(w http.ResponseWriter, r *http.Request, v interface{}) {
				actualPanic = v
			}),
	)
	ensure.Nil(t, err)
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	givenPanic := int(42)
	mux.GET(hr.URL.Path, func(http.ResponseWriter, *http.Request) error {
		panic(givenPanic)
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, actualPanic, givenPanic)
}

func TestHandlerNoPanic(t *testing.T) {
	mux, err := ctxmux.New(
		ctxmux.MuxPanicHandler(
			func(w http.ResponseWriter, r *http.Request, v interface{}) {
				panic("not reached")
			}),
	)
	ensure.Nil(t, err)
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.GET(hr.URL.Path, func(http.ResponseWriter, *http.Request) error {
		return nil
	})
	mux.ServeHTTP(hw, hr)
}

func TestHandlerNotFound(t *testing.T) {
	var called bool
	mux, err := ctxmux.New(
		ctxmux.MuxNotFoundHandler(
			func(w http.ResponseWriter, r *http.Request) error {
				called = true
				return nil
			}),
	)
	ensure.Nil(t, err)
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.ServeHTTP(hw, hr)
	ensure.True(t, called)
}

func TestRedirectTrailingSlash(t *testing.T) {
	mux, err := ctxmux.New(ctxmux.MuxRedirectTrailingSlash())
	ensure.Nil(t, err)
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/foo",
		},
	}
	mux.GET(hr.URL.Path+"/", func(http.ResponseWriter, *http.Request) error {
		return nil
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, hw.Header().Get("Location"), hr.URL.Path)
	ensure.DeepEqual(t, hw.Code, http.StatusMovedPermanently)
}
