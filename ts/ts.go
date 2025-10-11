package ts

import (
	"time"

	"github.com/jonboulle/clockwork"
)

// Clock wraps a clockwork.Clock so that the Now method is a little more convenient.
type Clock interface {
	Now() time.Time
}

// LocalTimeClock does just what all the other clocks do, but it returns local time truncated to the second.
type LocalTimeClock struct {
	clock clockwork.Clock
}

// NewRealClock gets a plain ol' real clock that tells local time, truncated to the second.
func NewRealClock() Clock {
	return &LocalTimeClock{
		clock: clockwork.NewRealClock(),
	}
}

// Now provides a timestamp truncated to the second, and in local time,
// convenient for human-readable times.
func (c *LocalTimeClock) Now() time.Time {
	return c.clock.Now().Local().Truncate(time.Second)
}

func (c *LocalTimeClock) Clockwork() clockwork.Clock {
	return c.clock
}
