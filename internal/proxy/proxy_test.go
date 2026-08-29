package proxy

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// captureLogs redirects the default slog logger to a buffer for the
// duration of a test, restoring the previous logger afterwards.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid http", input: "http://example.com", wantErr: false},
		{name: "valid https", input: "https://example.com:8443", wantErr: false},
		{name: "missing scheme", input: "example.com", wantErr: true},
		{name: "invalid scheme", input: "ftp://example.com", wantErr: true},
		{name: "missing host", input: "http://", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got.String() != tt.input {
				t.Fatalf("validateURL(%q) = %q, want %q", tt.input, got.String(), tt.input)
			}
		})
	}
}

func TestNewReverseProxyForwardsRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello from upstream"))
	}))
	defer upstream.Close()

	proxy, err := NewReverseProxy(upstream.URL)
	if err != nil {
		t.Fatalf("NewReverseProxy(%q) error = %v", upstream.URL, err)
	}

	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL)
	if err != nil {
		t.Fatalf("GET %s: %v", frontend.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusTeapot)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "hello from upstream" {
		t.Fatalf("body = %q, want %q", body, "hello from upstream")
	}
}

func TestNewReverseProxyRewritesHostHeader(t *testing.T) {
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer upstream.Close()
	upstreamHost := upstream.URL[len("http://"):]

	proxy, err := NewReverseProxy(upstream.URL)
	if err != nil {
		t.Fatalf("NewReverseProxy(%q) error = %v", upstream.URL, err)
	}

	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	if _, err := http.Get(frontend.URL); err != nil {
		t.Fatalf("GET %s: %v", frontend.URL, err)
	}

	if gotHost != upstreamHost {
		t.Fatalf("upstream saw Host = %q, want %q (the frontend's own host would break virtual-host routing on the real target)", gotHost, upstreamHost)
	}
}

func TestNewReverseProxyLogsForwardedRequests(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"error":"short and stout"}`))
	}))
	defer upstream.Close()

	proxy, err := NewReverseProxy(upstream.URL)
	if err != nil {
		t.Fatalf("NewReverseProxy(%q) error = %v", upstream.URL, err)
	}

	logs := captureLogs(t)
	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/hello")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != `{"error":"short and stout"}` {
		t.Fatalf("body = %q, want the untouched upstream body (logging must not consume it)", body)
	}

	out := logs.String()
	for _, want := range []string{"forwarded request", "path=/hello", "status=418", `body="{'error':'short and stout'}"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("log output = %q, want it to contain %q", out, want)
		}
	}
}

func TestTruncateBodyFlattensWhitespaceAndQuotes(t *testing.T) {
	got := truncateBody([]byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n"))
	want := `{ 'a': 1, 'b': 2 }`
	if got != want {
		t.Fatalf("truncateBody(...) = %q, want %q", got, want)
	}
}

func TestNewReverseProxyTruncatesLoggedBody(t *testing.T) {
	longBody := bytes.Repeat([]byte("a"), maxLoggedBodyBytes+50)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(longBody)
	}))
	defer upstream.Close()

	proxy, err := NewReverseProxy(upstream.URL)
	if err != nil {
		t.Fatalf("NewReverseProxy(%q) error = %v", upstream.URL, err)
	}

	logs := captureLogs(t)
	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if len(body) != len(longBody) {
		t.Fatalf("client received %d bytes, want %d (truncation must only affect the log, not the real response)", len(body), len(longBody))
	}

	if !bytes.Contains(logs.Bytes(), []byte("...(truncated)")) {
		t.Fatalf("log output = %q, want it to mark the body as truncated", logs.String())
	}
}

func TestNewReverseProxyLogsUpstreamFailures(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close() // closed immediately: target is now unreachable

	proxy, err := NewReverseProxy(upstreamURL)
	if err != nil {
		t.Fatalf("NewReverseProxy(%q) error = %v", upstreamURL, err)
	}

	logs := captureLogs(t)
	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/hello")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadGateway)
	}
	if out := logs.String(); !bytes.Contains([]byte(out), []byte("forwarding request failed")) {
		t.Fatalf("log output = %q, want it to contain %q", out, "forwarding request failed")
	}
}

func TestNewReverseProxyInvalidTarget(t *testing.T) {
	if _, err := NewReverseProxy("not-a-target"); err == nil {
		t.Fatalf("NewReverseProxy(%q) error = nil, want error", "not-a-target")
	}
}

func TestPickStatusRuleNoRules(t *testing.T) {
	_, ok := pickStatusRule(nil)
	if ok {
		t.Fatalf("pickStatusRule(nil) matched, want no match")
	}
}

func TestPickStatusRuleAlwaysMatches(t *testing.T) {
	rules := []StatusRule{{Code: 500, Probability: 1}}
	for i := 0; i < 100; i++ {
		got, ok := pickStatusRule(rules)
		if !ok {
			t.Fatalf("pickStatusRule(%v) = false, want true", rules)
		}
		if got.Code != 500 {
			t.Fatalf("pickStatusRule(%v) = %+v, want Code 500", rules, got)
		}
	}
}

func TestPickStatusRuleNeverMatches(t *testing.T) {
	rules := []StatusRule{{Code: 500, Probability: 0}}
	for i := 0; i < 100; i++ {
		if _, ok := pickStatusRule(rules); ok {
			t.Fatalf("pickStatusRule(%v) matched, want no match", rules)
		}
	}
}

func TestPickStatusRuleFirstRuleWinsWhenCertain(t *testing.T) {
	rules := []StatusRule{
		{Code: 500, Probability: 1},
		{Code: 503, Probability: 1},
	}
	for i := 0; i < 100; i++ {
		got, ok := pickStatusRule(rules)
		if !ok || got.Code != 500 {
			t.Fatalf("pickStatusRule(%v) = %+v, ok=%v, want Code 500, ok=true", rules, got, ok)
		}
	}
}
