package ctxmux_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/daaku/ctxmux"
	"github.com/facebookgo/ensure"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
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

func TestContextPipeChainSuccess(t *testing.T) {
	const key = int(1)
	const val = int(2)
	p := ctxmux.ContextPipeChain(
		func(ctx context.Context, r *http.Request) (context.Context, error) {
			return context.WithValue(ctx, key, val), nil
		})
	ctx, err := p(context.Background(), nil)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, ctx.Value(key), val)
}

func TestContextPipeChainFailure(t *testing.T) {
	givenErr := errors.New("")
	p := ctxmux.ContextPipeChain(
		func(context.Context, *http.Request) (context.Context, error) {
			return nil, givenErr
		})
	_, err := p(context.Background(), nil)
	ensure.DeepEqual(t, err, givenErr)
}

func TestWrapMethods(t *testing.T) {
	cases := []struct {
		Method   string
		Register func(*ctxmux.Mux, string, ctxmux.Handle)
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
		mux := ctxmux.Mux{
			ContextPipe: ctxmux.ContextPipeChain(
				func(ctx context.Context, r *http.Request) (context.Context, error) {
					return context.WithValue(ctx, key, val), nil
				}),
		}
		hw := httptest.NewRecorder()
		hr := &http.Request{
			Method: c.Method,
			URL: &url.URL{
				Path: "/",
			},
		}
		c.Register(&mux, hr.URL.Path, func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
			ensure.DeepEqual(t, ctx.Value(key), val)
			w.Write(body)
			return nil
		})
		mux.ServeHTTP(hw, hr)
		ensure.DeepEqual(t, hw.Body.Bytes(), body)
	}
}

func TestMuxContextPipeError(t *testing.T) {
	givenErr := errors.New("")
	var actualErr error
	mux := ctxmux.Mux{
		ContextPipe: func(context.Context, *http.Request) (context.Context, error) {
			return nil, givenErr
		},
		ErrorHandler: func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			actualErr = err
		},
	}
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.GET(hr.URL.Path, func(context.Context, http.ResponseWriter, *http.Request) error {
		panic("not reached")
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, actualErr, givenErr)
}

func TestHandleCustomMethod(t *testing.T) {
	var mux ctxmux.Mux
	const method = "FOO"
	body := []byte("body")
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: method,
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.Handle(method, hr.URL.Path, func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		w.Write(body)
		return nil
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, hw.Body.Bytes(), body)
}

func TestHTTPHandler(t *testing.T) {
	var mux ctxmux.Mux
	const method = "FOO"
	body := []byte("body")
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: method,
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.Handler(
		method,
		hr.URL.Path,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(body)
		}))
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, hw.Body.Bytes(), body)
}

func TestHTTPHandlerFunc(t *testing.T) {
	var mux ctxmux.Mux
	const method = "FOO"
	body := []byte("body")
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: method,
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.HandlerFunc(
		method, hr.URL.Path, func(w http.ResponseWriter, r *http.Request) {
			w.Write(body)
		})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, hw.Body.Bytes(), body)
}

func TestHandleReturnErr(t *testing.T) {
	givenErr := errors.New("")
	var actualErr error
	mux := ctxmux.Mux{
		ErrorHandler: func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			actualErr = err
		},
	}
	hw := httptest.NewRecorder()
	hr := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/",
		},
	}
	mux.GET(hr.URL.Path, func(context.Context, http.ResponseWriter, *http.Request) error {
		return givenErr
	})
	mux.ServeHTTP(hw, hr)
	ensure.DeepEqual(t, actualErr, givenErr)
}
