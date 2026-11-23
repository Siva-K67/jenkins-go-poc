package consent

import (
	"testing"
	"time"
)

func TestIsValid(t *testing.T) {
	now := time.Now()

	// Each case has a name, an expiry time, and the answer we expect
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"future expiry is valid", now.Add(time.Hour), true},
		{"past expiry is invalid", now.Add(-time.Hour), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsValid(tc.expiresAt, now)
			if got != tc.want {
				t.Errorf("IsValid() = %v, want %v", got, tc.want)
			}
		})
	}
}
