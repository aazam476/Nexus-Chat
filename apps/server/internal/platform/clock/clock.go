// Package clock lets code ask for the current time through an interface, so
// tests can control time instead of sleeping.
//
// Production code receives [System]; tests pass [Fixed] or their own fake.
package clock

import "time"

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// System is the real clock. It always returns UTC so stored and logged
// timestamps never depend on the host's time zone.
type System struct{}

// Now returns the current UTC time.
func (System) Now() time.Time {
	return time.Now().UTC()
}

// Fixed is a clock that always returns the same instant. Use it in tests.
type Fixed struct {
	Time time.Time
}

// Now returns the fixed instant.
func (fixed Fixed) Now() time.Time {
	return fixed.Time
}
