package headers

import (
	"net/http"
	"strings"

	"bitbucket.org/lightcodelabs/caddy2"
	"bitbucket.org/lightcodelabs/caddy2/modules/caddyhttp"
)

func init() {
	caddy2.RegisterModule(caddy2.Module{
		Name: "http.middleware.headers",
		New:  func() interface{} { return new(Headers) },
	})
}

// Headers is a middleware which can mutate HTTP headers.
type Headers struct {
	Request  *HeaderOps     `json:"request,omitempty"`
	Response *RespHeaderOps `json:"response,omitempty"`
}

// HeaderOps defines some operations to
// perform on HTTP headers.
type HeaderOps struct {
	Add    http.Header `json:"add,omitempty"`
	Set    http.Header `json:"set,omitempty"`
	Delete []string    `json:"delete,omitempty"`
}

// RespHeaderOps is like HeaderOps, but
// optionally deferred until response time.
type RespHeaderOps struct {
	*HeaderOps
	Require  *caddyhttp.ResponseMatcher `json:"require,omitempty"`
	Deferred bool                       `json:"deferred,omitempty"`
}

func (h Headers) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	repl := r.Context().Value(caddy2.ReplacerCtxKey).(caddy2.Replacer)
	apply(h.Request, r.Header, repl)
	if h.Response.Deferred || h.Response.Require != nil {
		w = &responseWriterWrapper{
			ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{ResponseWriter: w},
			replacer:              repl,
			require:               h.Response.Require,
			headerOps:             h.Response.HeaderOps,
		}
	} else {
		apply(h.Response.HeaderOps, w.Header(), repl)
	}
	return next.ServeHTTP(w, r)
}

func apply(ops *HeaderOps, hdr http.Header, repl caddy2.Replacer) {
	for fieldName, vals := range ops.Add {
		fieldName = repl.ReplaceAll(fieldName, "")
		for _, v := range vals {
			hdr.Add(fieldName, repl.ReplaceAll(v, ""))
		}
	}
	for fieldName, vals := range ops.Set {
		fieldName = repl.ReplaceAll(fieldName, "")
		for i := range vals {
			vals[i] = repl.ReplaceAll(vals[i], "")
		}
		hdr.Set(fieldName, strings.Join(vals, ","))
	}
	for _, fieldName := range ops.Delete {
		hdr.Del(repl.ReplaceAll(fieldName, ""))
	}
}

// responseWriterWrapper defers response header
// operations until WriteHeader is called.
type responseWriterWrapper struct {
	*caddyhttp.ResponseWriterWrapper
	replacer    caddy2.Replacer
	require     *caddyhttp.ResponseMatcher
	headerOps   *HeaderOps
	wroteHeader bool
}

func (rww *responseWriterWrapper) Write(d []byte) (int, error) {
	if !rww.wroteHeader {
		rww.WriteHeader(http.StatusOK)
	}
	return rww.ResponseWriterWrapper.Write(d)
}

func (rww *responseWriterWrapper) WriteHeader(status int) {
	if rww.wroteHeader {
		return
	}
	rww.wroteHeader = true
	if rww.require == nil || rww.require.Match(status, rww.ResponseWriterWrapper.Header()) {
		apply(rww.headerOps, rww.ResponseWriterWrapper.Header(), rww.replacer)
	}
	rww.ResponseWriterWrapper.WriteHeader(status)
}

// Interface guards
var (
	_ caddyhttp.MiddlewareHandler = (*Headers)(nil)
	_ caddyhttp.HTTPInterfaces    = (*responseWriterWrapper)(nil)
)