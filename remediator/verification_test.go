package main

import "testing"

func TestVerificationResult(t *testing.T) {
	tests := []struct {
		name   string
		before []ObservedMetric
		after  []ObservedMetric
		want   string
	}{
		{
			name:   "improved",
			before: []ObservedMetric{{Name: "gRPC error ratio (5m)", Value: "0.20"}},
			after:  []ObservedMetric{{Name: "gRPC error ratio (5m)", Value: "0.01"}},
			want:   "improved",
		},
		{
			name:   "not improved",
			before: []ObservedMetric{{Name: "HTTP 5xx ratio (5m)", Value: "0.10"}},
			after:  []ObservedMetric{{Name: "HTTP 5xx ratio (5m)", Value: "0.20"}},
			want:   "not_improved",
		},
		{
			name:   "no baseline",
			before: nil,
			after:  []ObservedMetric{{Name: "HTTP 5xx ratio (5m)", Value: "0.01"}},
			want:   "no_baseline",
		},
		{
			name:   "no after data",
			before: []ObservedMetric{{Name: "HTTP 5xx ratio (5m)", Value: "0.10"}},
			after:  nil,
			want:   "no_after_data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := verificationResult(tt.before, tt.after); got != tt.want {
				t.Fatalf("verificationResult() = %q, want %q", got, tt.want)
			}
		})
	}
}
