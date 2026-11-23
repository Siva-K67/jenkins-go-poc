package consent

import "time"

// IsValid returns true if the consent has not expired yet.
func IsValid(expiresAt, now time.Time) bool {
	return now.Before(expiresAt)
}
