package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
