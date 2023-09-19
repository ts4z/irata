package ts

import (
	"time"
)

// Now provides a timestamp truncated to the second, and in local time,
// convenient for human-readable times.
func Now() time.Time {
	return time.Now().Local().Truncate(time.Second)
}
