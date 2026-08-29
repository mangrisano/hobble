package proxy

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type StatusRule struct {
	Code        int
	Probability float64
}

type LatencyRange struct {
	Min time.Duration
	Max time.Duration
}

func ParseStatusRule(s string) (StatusRule, error) {
	codeStr, probabilityStr, found := strings.Cut(s, "=")
	if !found {
		return StatusRule{}, fmt.Errorf("invalid status rule %q: missing '='", s)
	}
	code, err := strconv.Atoi(codeStr)
	if err != nil {
		return StatusRule{}, fmt.Errorf("invalid status rule %q: %v", s, err)
	}
	if http.StatusText(code) == "" {
		return StatusRule{}, fmt.Errorf("invalid status code: %q", s)
	}
	probability, err := strconv.ParseFloat(probabilityStr, 64)
	if err != nil {
		return StatusRule{}, fmt.Errorf("invalid status rule %q: %v", s, err)
	}
	if probability < 0 || probability > 1 {
		return StatusRule{}, fmt.Errorf("invalid probability %q", s)
	}
	return StatusRule{Code: code, Probability: probability}, nil
}

func ParseLatencyRange(s string) (LatencyRange, error) {
	minStr, maxStr, found := strings.Cut(s, "-")
	min, err := time.ParseDuration(minStr)
	if err != nil {
		return LatencyRange{}, fmt.Errorf("invalid latency %q: %v", s, err)
	}
	if !found {
		return LatencyRange{Min: min, Max: min}, nil
	}
	max, err := time.ParseDuration(maxStr)
	if err != nil {
		return LatencyRange{}, fmt.Errorf("invalid latency range %q: %v", s, err)
	}
	if max < min {
		return LatencyRange{}, fmt.Errorf("invalid latency range %q: max is less than min", s)
	}
	return LatencyRange{Min: min, Max: max}, nil
}
