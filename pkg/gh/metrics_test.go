package gh

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/m4ns0ur/httpcache"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type stubTransport struct {
	resp *http.Response
	err  error
}

func (s *stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.resp != nil && s.resp.Request == nil {
		s.resp.Request = req
	}
	return s.resp, s.err
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestInstrumentedTransportMetrics(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/org/repo", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	path := req.URL.Path
	method := req.Method

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString("ok")),
	}

	transport := newInstrumentedTransport(&stubTransport{resp: resp})

	baseReq := testutil.ToFloat64(githubAPIRequestsTotal.WithLabelValues(path, method, "2xx"))
	baseCache := testutil.ToFloat64(githubAPICacheTotal.WithLabelValues(path, method, "miss"))
	baseInflight := testutil.ToFloat64(githubAPIInflight.WithLabelValues(path, method))

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}

	if got := testutil.ToFloat64(githubAPIRequestsTotal.WithLabelValues(path, method, "2xx")); got != baseReq+1 {
		t.Fatalf("requests_total mismatch: got=%v want=%v", got, baseReq+1)
	}
	if got := testutil.ToFloat64(githubAPICacheTotal.WithLabelValues(path, method, "miss")); got != baseCache+1 {
		t.Fatalf("cache_total miss mismatch: got=%v want=%v", got, baseCache+1)
	}
	if got := testutil.ToFloat64(githubAPIInflight.WithLabelValues(path, method)); got != baseInflight {
		t.Fatalf("inflight mismatch: got=%v want=%v", got, baseInflight)
	}
}

func TestInstrumentedTransportCacheHit(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/org/repo", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	path := req.URL.Path
	method := req.Method

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString("cached")),
	}
	resp.Header.Set(httpcache.XFromCache, "1")

	transport := newInstrumentedTransport(&stubTransport{resp: resp})

	baseCache := testutil.ToFloat64(githubAPICacheTotal.WithLabelValues(path, method, "hit"))

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}

	if got := testutil.ToFloat64(githubAPICacheTotal.WithLabelValues(path, method, "hit")); got != baseCache+1 {
		t.Fatalf("cache_total hit mismatch: got=%v want=%v", got, baseCache+1)
	}
}

func TestInstrumentedTransportErrorMetrics(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/org/repo", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	path := req.URL.Path
	method := req.Method

	transport := newInstrumentedTransport(&stubTransport{err: timeoutErr{}})

	baseReq := testutil.ToFloat64(githubAPIRequestsTotal.WithLabelValues(path, method, "error"))
	baseErr := testutil.ToFloat64(githubAPIErrorsTotal.WithLabelValues(path, method, "timeout"))

	if _, err := transport.RoundTrip(req); err == nil {
		t.Fatal("expected error, got nil")
	}

	if got := testutil.ToFloat64(githubAPIRequestsTotal.WithLabelValues(path, method, "error")); got != baseReq+1 {
		t.Fatalf("requests_total error mismatch: got=%v want=%v", got, baseReq+1)
	}
	if got := testutil.ToFloat64(githubAPIErrorsTotal.WithLabelValues(path, method, "timeout")); got != baseErr+1 {
		t.Fatalf("errors_total mismatch: got=%v want=%v", got, baseErr+1)
	}
}
