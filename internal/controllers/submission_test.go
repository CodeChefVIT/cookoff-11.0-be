package controllers

import "testing"

func TestAttemptAllowsSubmission(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{"available is not purchased", "available", false},
		{"empty status is not purchased", "", false},
		{"bought is purchased", "bought", true},
		{"answered is purchased", "answered", true},
		{"unknown status is rejected", "refunded", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := attemptAllowsSubmission(tc.status); got != tc.want {
				t.Errorf("attemptAllowsSubmission(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
