package ts

import (
	"time"

	"github.com/jonboulle/clockwork"
)

// Clock wraps Clock so that the Now method is a little more convenient.
type Clock struct {
	realClock clockwork.Clock
}

func NewRealClock() *Clock {
	return &Clock{
		realClock: clockwork.NewRealClock(),
	}
}

// Now provides a timestamp truncated to the second, and in local time,
// convenient for human-readable times.
func (c *Clock) Now() time.Time {
	return c.realClock.Now().Local().Truncate(time.Second)
}

func (c *Clock) RealClock() clockwork.Clock {
	return c.realClock
}
