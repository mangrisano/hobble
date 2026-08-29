package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func NewReverseProxy(target string) (http.Handler, error) {
	result, err := validateURL(target)
	if err != nil {
		return nil, err
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(result)
		},
		ModifyResponse: func(resp *http.Response) error {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response body: %w", err)
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			slog.Info("forwarded request", "path", resp.Request.URL.Path, "status", resp.StatusCode, "body", truncateBody(body))
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("forwarding request failed", "path", r.URL.Path, "error", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	return rp, nil
}

// maxLoggedBodyBytes caps how much of a response body ends up in a log
// line, so a bug report is self-contained without flooding the log.
const maxLoggedBodyBytes = 500

// truncateBody renders body as a single-line string suitable for a log
// line: whitespace collapsed so a pretty-printed JSON/HTML body doesn't turn
// into a wall of escaped "\n", cut at maxLoggedBodyBytes and marked as
// truncated if it was longer.
func truncateBody(body []byte) string {
	flattened := strings.Join(strings.Fields(string(body)), " ")
	// slog.TextHandler quotes the whole value and backslash-escapes any
	// double quote inside it; JSON/HTML bodies are full of those, so swap
	// them for single quotes to keep the log line readable.
	flattened = strings.ReplaceAll(flattened, `"`, "'")
	if len(flattened) <= maxLoggedBodyBytes {
		return flattened
	}
	return flattened[:maxLoggedBodyBytes] + "...(truncated)"
}

func validateURL(u string) (*url.URL, error) {
	target, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid target %q: %v", u, err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("invalid target %q: scheme must be http or https", u)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("invalid target %q: missing host", u)
	}
	return target, nil
}

func dropConnection(w http.ResponseWriter) bool {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		slog.Warn("cannot drop connection: response writer does not support hijacking")
		return false
	}
	tcpConn, _, err := hijacker.Hijack()
	if err != nil {
		slog.Warn("cannot drop connection: hijack failed", "error", err)
		return false
	}
	if err := tcpConn.Close(); err != nil {
		slog.Warn("error closing hijacked connection", "error", err)
	}
	return true
}

func pickStatusRule(rules []StatusRule) (StatusRule, bool) {
	n := rand.Float64()
	cumulative := 0.0
	for _, rule := range rules {
		currentCumulative := cumulative
		cumulative += rule.Probability
		if n >= float64(currentCumulative) && n <= float64(cumulative) {
			return StatusRule{Code: rule.Code, Probability: rule.Probability}, true
		}
	}
	return StatusRule{}, false
}

func WithFaults(next http.Handler, statusRules []StatusRule, latency LatencyRange, dropProbability float64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := rand.Float64()
		if n < dropProbability {
			dropped := dropConnection(w)
			slog.Info("dropping connection", "path", r.URL.Path, "hijacked", dropped)
			return
		}
		rule, ok := pickStatusRule(statusRules)
		if ok {
			slog.Info("injecting status", "path", r.URL.Path, "code", rule.Code, "probability", rule.Probability, "body", "")
			w.WriteHeader(rule.Code)
			return
		}
		delta := latency.Max - latency.Min
		extra := time.Duration(rand.Int64N(int64(delta) + 1))
		sleep := latency.Min + extra
		slog.Debug("injecting latency", "path", r.URL.Path, "duration", sleep)
		time.Sleep(sleep)
		slog.Debug("forwarding request", "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
