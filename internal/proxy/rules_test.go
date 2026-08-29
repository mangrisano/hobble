package proxy

import (
	"testing"
	"time"
)

func TestParseStatusRule(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    StatusRule
		wantErr bool
	}{
		{"valid rule", "500=0.1", StatusRule{Code: 500, Probability: 0.1}, false},
		{"probability at zero", "404=0", StatusRule{Code: 404, Probability: 0}, false},
		{"probability at one", "503=1", StatusRule{Code: 503, Probability: 1}, false},
		{"missing separator", "500", StatusRule{}, true},
		{"non-numeric status code", "abc=0.1", StatusRule{}, true},
		{"unknown status code", "999=0.1", StatusRule{}, true},
		{"non-numeric probability", "500=abc", StatusRule{}, true},
		{"probability below zero", "500=-0.1", StatusRule{}, true},
		{"probability above one", "500=1.1", StatusRule{}, true},
		{"empty string", "", StatusRule{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseStatusRule(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseStatusRule(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("ParseStatusRule(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseLatencyRange(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    LatencyRange
		wantErr bool
	}{
		{"fixed value", "200ms", LatencyRange{Min: 200 * time.Millisecond, Max: 200 * time.Millisecond}, false},
		{"range", "200ms-800ms", LatencyRange{Min: 200 * time.Millisecond, Max: 800 * time.Millisecond}, false},
		{"range with equal bounds", "200ms-200ms", LatencyRange{Min: 200 * time.Millisecond, Max: 200 * time.Millisecond}, false},
		{"max less than min", "800ms-200ms", LatencyRange{}, true},
		{"invalid fixed duration", "abc", LatencyRange{}, true},
		{"invalid min duration", "abc-800ms", LatencyRange{}, true},
		{"invalid max duration", "200ms-abc", LatencyRange{}, true},
		{"trailing dash with no max", "200ms-", LatencyRange{}, true},
		{"empty string", "", LatencyRange{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLatencyRange(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseLatencyRange(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("ParseLatencyRange(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}
