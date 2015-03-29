// Package ctxmux provides an opinionated mux. It builds on the context
// library and combines it with the httprouter library known for it's
// performance. The equivalent of the ServeHTTP in ctxmux is:
//
//    ServeHTTP(ctx context.Context, w http.ResponseWriter, r *http.Request) error
//
// It provides a hook to control context creation when a request arrives.
// Additionally an error can be returned which is passed thru to the error
// handler. The error handler is responsible for sending a response and
// possibly logging it as necessary. Similarly panics are also handled and
// passed to the panic handler.
package ctxmux

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

type contextParamsKeyT int

var contextParamsKey = contextParamsKeyT(0)

// WithParams returns a new context.Context instance with the params included.
func WithParams(ctx context.Context, p httprouter.Params) context.Context {
	return context.WithValue(ctx, contextParamsKey, p)
}

// ContextParams extracts out the params from the context if possible.
func ContextParams(ctx context.Context) httprouter.Params {
	p, _ := ctx.Value(contextParamsKey).(httprouter.Params)
	return p
}

// HTTPHandler calls the underlying http.Handler.
func HTTPHandler(handler http.Handler) Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		handler.ServeHTTP(w, r)
		return nil
	}
}

// HTTPHandlerFunc calls the underlying http.HandlerFunc.
func HTTPHandlerFunc(handler http.HandlerFunc) Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
		handler(w, r)
		return nil
	}
}

// Handler is an augmented http.Handler.
type Handler func(ctx context.Context, w http.ResponseWriter, r *http.Request) error

// Mux provides shared context initialization and error handling.
type Mux struct {
	contextMaker func(*http.Request) (context.Context, error)
	errorHandler ErrorHandler
	panicHandler PanicHandler
	r            httprouter.Router
}

func (m *Mux) makeContext(r *http.Request) (context.Context, error) {
	if m.contextMaker != nil {
		ctx, err := m.contextMaker(r)
		if err != nil {
			return context.Background(), err
		}
		return ctx, nil
	}
	return context.Background(), nil
}

func (m *Mux) wrap(handler Handler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var ctx context.Context // so the panicHandler can get to it

		if m.panicHandler != nil {
			defer func() {
				if v := recover(); v != nil {
					m.panicHandler(ctx, w, r, v)
				}
			}()
		}

		ctx, err := m.makeContext(r)
		if err != nil {
			m.errorHandler(ctx, w, r, err)
			return
		}
		ctx = WithParams(ctx, p)

		if err := handler(ctx, w, r); err != nil {
			m.errorHandler(ctx, w, r, err)
			return
		}
	}
}

// Handler by method and path.
func (m *Mux) Handler(method, path string, handler Handler) {
	m.r.Handle(method, path, m.wrap(handler))
}

// HEAD methods at path.
func (m *Mux) HEAD(path string, handler Handler) {
	m.r.HEAD(path, m.wrap(handler))
}

// GET methods at path.
func (m *Mux) GET(path string, handler Handler) {
	m.r.GET(path, m.wrap(handler))
}

// POST methods at path.
func (m *Mux) POST(path string, handler Handler) {
	m.r.POST(path, m.wrap(handler))
}

// PUT methods at path.
func (m *Mux) PUT(path string, handler Handler) {
	m.r.PUT(path, m.wrap(handler))
}

// DELETE methods at path.
func (m *Mux) DELETE(path string, handler Handler) {
	m.r.DELETE(path, m.wrap(handler))
}

// PATCH methods at path.
func (m *Mux) PATCH(path string, handler Handler) {
	m.r.PATCH(path, m.wrap(handler))
}

// ServeHTTP allows Mux to be used as a http.Handler.
func (m *Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	m.r.ServeHTTP(w, req)
}

// MuxOption are used to set various mux options.
type MuxOption func(*Mux) error

// MuxContextMaker sets a function to create the context for each request. The
// default context is context.Background(). If the assigned function returns an
// error, it will be passed to the ErrorHandler.
func MuxContextMaker(f func(*http.Request) (context.Context, error)) MuxOption {
	return func(m *Mux) error {
		m.contextMaker = f
		return nil
	}
}

// ErrorHandler is invoked with errors returned by handler functions.
type ErrorHandler func(context.Context, http.ResponseWriter, *http.Request, error)

// MuxErrorHandler configures the ErrorHandler for the Mux. If one isn't set,
// the default behaviour is to log it and send a static error message of
// "internal server error".
func MuxErrorHandler(handler ErrorHandler) MuxOption {
	return func(m *Mux) error {
		m.errorHandler = handler
		return nil
	}
}

// PanicHandler is invoked with the panics that occur during context creation
// or while the handler is running.
type PanicHandler func(context.Context, http.ResponseWriter, *http.Request, interface{})

// MuxPanicHandler configures the panic handler for the Mux. If one is not
// configured, the default behavior is what the net/http package does; which is
// to print a trace and ignore it.
func MuxPanicHandler(handler PanicHandler) MuxOption {
	return func(m *Mux) error {
		m.panicHandler = handler
		return nil
	}
}

// MuxNotFoundHandler configures a Handler that is invoked for requests where
// one isn't found.
func MuxNotFoundHandler(handler Handler) MuxOption {
	return func(m *Mux) error {
		h := m.wrap(handler)
		m.r.NotFound = func(w http.ResponseWriter, r *http.Request) {
			h(w, r, nil)
		}
		return nil
	}
}

// MuxRedirectTrailingSlash configures the Mux to automatically handling
// missing or extraneous trailing slashes by redirecting.
func MuxRedirectTrailingSlash(m *Mux) error {
	m.r.RedirectTrailingSlash = true
	return nil
}

// New creates a new Mux and configures it with the given options.
func New(options ...MuxOption) (*Mux, error) {
	var m Mux
	for _, o := range options {
		if err := o(&m); err != nil {
			return nil, err
		}
	}
	return &m, nil
}
