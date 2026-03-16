// Package chop defines the interface for chop algorithms.
package chop

// A Chopper computes a division of prizes among players based on chip counts.
type Chopper interface {
	// Chop takes chip counts and prize amounts (both indexed by position),
	// and returns the chopped payout for each player. The sum of the returned
	// values should equal the sum of prizes.
	Chop(chips []int, prizes []int) ([]int, error)

	// MaxPlayers returns the maximum number of players this algorithm supports.
	MaxPlayers() int
}
