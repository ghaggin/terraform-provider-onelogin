package util

import "time"

// GetTimestampString returns a string representation of the current time in a standard format
func GetTimestampString() string {
	return time.Now().UTC().Format(time.RFC3339)
}
