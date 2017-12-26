package prisma

import (
	"fmt"
	"time"
	"net/http"

	"github.com/slover2000/prisma/trace"
)

// Transport is an http.RoundTripper that traces the outgoing requests.
//
// Transport is safe for concurrent usage.
type Transport struct {
	Client 	*InterceptorClient
	Base    http.RoundTripper
}

// RoundTrip creates a trace.Span and inserts it into the outgoing request's headers.
// The created span can follow a parent span, if a parent is presented in
// the request's context.
func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	span := trace.FromContext(req.Context()).NewRemoteChild(req)
	startTime := time.Now()
	resp, err := t.base().RoundTrip(req)

	// TODO(jbd): Is it possible to defer the span.Finish?
	// In cases where RoundTrip panics, we still can finish the span.
	span.Finish(trace.WithResponse(resp))

	// log http request
	statusCode := 500
	if resp != nil {
		statusCode = resp.StatusCode
	}

	// do metrics
	if t.Client != nil && t.Client.httpClientMetrics != nil {
		t.Client.httpClientMetrics.CounterHTTP(req, time.Since(startTime), statusCode)
	}	

	// log request
	if t.Client != nil && t.Client.log != nil {
		msg := fmt.Sprintf("%s %s %s %d", req.Method, req.URL.String(), req.Proto, statusCode)
		t.Client.log.LogHttpClientLine(req, startTime, statusCode, msg)	
	}
	return resp, err
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t Transport) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := t.base().(canceler); ok {
		cr.CancelRequest(req)
	}
}

func (t Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

type withStatusCodeResponseWriter struct {
	writer     http.ResponseWriter
	statusCode int
}

func (w *withStatusCodeResponseWriter) Header() http.Header {
	return w.writer.Header()
}

func (w *withStatusCodeResponseWriter) Write(bytes []byte) (int, error) {
	return w.writer.Write(bytes)
}

func (w *withStatusCodeResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.writer.WriteHeader(code)
}

// HTTPHandler returns a http.Handler from the given handler
// that is aware of the incoming request's span.
// The span can be extracted from the incoming request in handler
// functions from incoming request's context:
//
//    span := trace.FromContext(r.Context())
//
// The span will be auto finished by the handler.
func (c *InterceptorClient) HTTPHandler(h http.Handler) http.Handler {
	if c == nil {
		return h
	}
	return &handler{client: c, handler: h}
}

type handler struct {
	client 	*InterceptorClient
	handler http.Handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var span *trace.Span
	if h.client != nil && h.client.trace != nil {
		span = h.client.trace.SpanFromRequestOrNot(r)
	}	
	defer span.Finish()

	r = r.WithContext(trace.NewContext(r.Context(), span))
	// if ok && !optionsOk {
	// 	// Inject the trace context back to the response with the sampling options.
	// 	// TODO(jbd): Remove when there is a better way to report the client's sampling.
	// 	w.Header().Set(httpTraceHeader, spanHeader(traceID, parentSpanID, span.trace.localOptions))
	// }
	
	rw := &withStatusCodeResponseWriter{writer: w}
	startTime := time.Now()
	h.handler.ServeHTTP(rw, r)

	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}	

	// do metrics
	if h.client != nil && h.client.httpServerMetrics != nil {
		h.client.httpServerMetrics.CounterHTTP(r, time.Since(startTime), rw.statusCode)
	}

	// log request
	if h.client != nil && h.client.log != nil {
		msg := fmt.Sprintf("%s %s %s %d", r.Method, r.URL.String(), r.Proto, rw.statusCode)
		h.client.log.LogHttpServerLine(r, startTime, rw.statusCode, msg)
	}
}
