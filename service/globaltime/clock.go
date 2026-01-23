package globaltime

import "time"


type Clock interface {
	Now() time.Time
}

// SystemClock returns real system time.
type SystemClock struct{}

// NewSystemClock creates a new clock that returns time.Now().UTC().
func NewSystemClock() Clock {
	return SystemClock{}
}

func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
