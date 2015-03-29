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

// Handle is an augmented http.Handler.
type Handle func(ctx context.Context, w http.ResponseWriter, r *http.Request) error

// ContextPipe pipes a context through, possibly adding something to it.
type ContextPipe func(ctx context.Context, r *http.Request) (context.Context, error)

// ContextPipeChain chains a series of ContextPipe.
func ContextPipeChain(pipes ...ContextPipe) ContextPipe {
	return func(ctx context.Context, r *http.Request) (context.Context, error) {
		for _, p := range pipes {
			ctxNew, err := p(ctx, r)
			if err != nil {
				return ctx, err
			}
			ctx = ctxNew
		}
		return ctx, nil
	}
}

// Mux provides shared context initialization and error handling.
type Mux struct {
	contextPipe  ContextPipe
	errorHandler func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error)
	r            httprouter.Router
}

func (m *Mux) wrap(handle Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := WithParams(context.Background(), p)
		if m.contextPipe != nil {
			ctxNew, err := m.contextPipe(ctx, r)
			if err != nil {
				m.errorHandler(ctx, w, r, err)
				return
			}
			ctx = ctxNew
		}

		if err := handle(ctx, w, r); err != nil {
			m.errorHandler(ctx, w, r, err)
			return
		}
	}
}

// Handle by method and path.
func (m *Mux) Handle(method, path string, handle Handle) {
	m.r.Handle(method, path, m.wrap(handle))
}

// HEAD methods at path.
func (m *Mux) HEAD(path string, handle Handle) {
	m.r.HEAD(path, m.wrap(handle))
}

// GET methods at path.
func (m *Mux) GET(path string, handle Handle) {
	m.r.GET(path, m.wrap(handle))
}

// POST methods at path.
func (m *Mux) POST(path string, handle Handle) {
	m.r.POST(path, m.wrap(handle))
}

// PUT methods at path.
func (m *Mux) PUT(path string, handle Handle) {
	m.r.PUT(path, m.wrap(handle))
}

// DELETE methods at path.
func (m *Mux) DELETE(path string, handle Handle) {
	m.r.DELETE(path, m.wrap(handle))
}

// PATCH methods at path.
func (m *Mux) PATCH(path string, handle Handle) {
	m.r.PATCH(path, m.wrap(handle))
}

// ServeHTTP allows Mux to be used as a http.Handler.
func (m *Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	m.r.ServeHTTP(w, req)
}

// Handler allows for registering a http.Handler.
func (m *Mux) Handler(method, path string, handler http.Handler) {
	m.r.Handler(method, path, handler)
}

// HandlerFunc allows for registering a http.HandlerFunc.
func (m *Mux) HandlerFunc(method, path string, handler http.HandlerFunc) {
	m.r.HandlerFunc(method, path, handler)
}

// MuxOption are used to set various mux options.
type MuxOption func(*Mux) error

// MuxContextPipe sets the ContextPipe function on the Mux. The default context
// is context.Background().
func MuxContextPipe(pipe ContextPipe) MuxOption {
	return func(m *Mux) error {
		m.contextPipe = pipe
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
//
// TODO: Should also be passed the context.Context.
type PanicHandler func(http.ResponseWriter, *http.Request, interface{})

// MuxPanicHandler configures the panic handler for the Mux. If one is not
// configured, the default behavior is what the net/http package does; which is
// to print a trace and ignore it.
func MuxPanicHandler(handler PanicHandler) MuxOption {
	return func(m *Mux) error {
		m.r.PanicHandler = handler
		return nil
	}
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
