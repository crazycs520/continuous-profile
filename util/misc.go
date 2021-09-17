package util

import "time"

// Millisecond returns the millisecond timestamp of the input time.
func Millisecond(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/int64(time.Millisecond)
}
