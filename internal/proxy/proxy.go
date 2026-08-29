package proxy

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"net/url"
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
			slog.Info("forwarded request", "path", resp.Request.URL.Path, "status", resp.StatusCode)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Warn("forwarding request failed", "path", r.URL.Path, "error", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	return rp, nil
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
			slog.Info("injecting status", "path", r.URL.Path, "code", rule.Code, "probability", rule.Probability)
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
