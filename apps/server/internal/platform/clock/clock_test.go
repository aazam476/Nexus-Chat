package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSystemReturnsCurrentUTCTime(t *testing.T) {
	before := time.Now()

	now := System{}.Now()

	assert.Equal(t, time.UTC, now.Location())
	assert.False(t, now.Before(before.Add(-time.Second)))
	assert.False(t, now.After(time.Now().Add(time.Second)))
}

func TestFixedReturnsItsTime(t *testing.T) {
	instant := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, instant, Fixed{Time: instant}.Now())
}
