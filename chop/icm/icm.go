// Package icm implements the Independent Chip Model for tournament poker.
//
// ICM computes each player's equity (expected dollar value) based on their
// chip stack and the remaining prize structure, using the Malmuth-Harville
// probability model.
//
// Reference: https://en.wikipedia.org/wiki/Independent_Chip_Model
package icm

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sort"
)

const MaxPlayers = 20

type Chopper struct{}

func (c *Chopper) Name() string {
	return "ICM"
}

func (c *Chopper) Chop(chips []int, prizes []int) ([]int, error) {
	return Chop(chips, prizes)
}

func (c *Chopper) MaxPlayers() int {
	return MaxPlayers
}

// CalculateEquity computes ICM equity for each remaining player.
//
// chips[i] is the chip count for player i (must be positive).
// prizes[k] is the prize for finishing in position k+1 (0-indexed;
// prizes[0] = 1st place prize). If len(prizes) < len(chips), positions
// beyond len(prizes) receive $0.
//
// Returns the expected dollar value for each player. The sum of returned
// equities equals the sum of prizes.
func CalculateEquity(chips []int, prizes []int) ([]float64, error) {
	n := len(chips)
	if n == 0 {
		return nil, errors.New("icm: no players")
	}
	if n > MaxPlayers {
		return nil, fmt.Errorf("icm: too many players (max %d)", MaxPlayers)
	}
	for i, c := range chips {
		if c <= 0 {
			return nil, fmt.Errorf("icm: player %d has non-positive chip count (%d)", i, c)
		}
	}

	// Pad prizes to length n (unspecified positions get $0).
	paddedPrizes := make([]float64, n)
	for i := 0; i < len(prizes) && i < n; i++ {
		paddedPrizes[i] = float64(prizes[i])
	}

	fullSet := (1 << n) - 1

	// Precompute chip totals for each subset using DP.
	totals := make([]float64, 1<<n)
	for mask := 1; mask <= fullSet; mask++ {
		lowest := mask & (-mask)
		rest := mask ^ lowest
		player := bits.TrailingZeros(uint(lowest))
		totals[mask] = totals[rest] + float64(chips[player])
	}

	// Memoization table: memo[player][mask] = equity of player given active set mask.
	memo := make([][]float64, n)
	computed := make([][]bool, n)
	for i := 0; i < n; i++ {
		memo[i] = make([]float64, 1<<n)
		computed[i] = make([]bool, 1<<n)
	}

	// equity computes the expected value for player within the active set mask.
	//
	// The Malmuth-Harville model assigns:
	//   P(player wins | set S) = chips[player] / totalChips(S)
	//
	// A player's equity in state S is:
	//   EV = P(wins) * prize_for_this_position
	//        + sum over other players j: P(j wins) * EV(player, S \ {j})
	var equity func(player int, mask int) float64
	equity = func(player int, mask int) float64 {
		if computed[player][mask] {
			return memo[player][mask]
		}
		computed[player][mask] = true

		size := bits.OnesCount(uint(mask))
		prizeIdx := n - size
		prize := paddedPrizes[prizeIdx]

		if size == 1 {
			memo[player][mask] = prize
			return prize
		}

		total := totals[mask]
		result := (float64(chips[player]) / total) * prize

		for j := 0; j < n; j++ {
			if j == player || mask&(1<<j) == 0 {
				continue
			}
			probJ := float64(chips[j]) / total
			result += probJ * equity(player, mask&^(1<<j))
		}

		memo[player][mask] = result
		return result
	}

	equities := make([]float64, n)
	for i := 0; i < n; i++ {
		equities[i] = equity(i, fullSet)
	}

	return equities, nil
}

// Chop computes ICM equities and rounds them to whole integer amounts,
// ensuring the total equals the sum of prizes. Rounding distributes
// remainders to players with the largest fractional parts.
func Chop(chips []int, prizes []int) ([]int, error) {
	equities, err := CalculateEquity(chips, prizes)
	if err != nil {
		return nil, err
	}

	totalPrizes := 0
	for _, p := range prizes {
		totalPrizes += p
	}

	n := len(equities)
	result := make([]int, n)
	allocated := 0

	type indexedFrac struct {
		index int
		frac  float64
	}
	fracs := make([]indexedFrac, n)

	for i, eq := range equities {
		floored := int(math.Floor(eq))
		result[i] = floored
		allocated += floored
		fracs[i] = indexedFrac{i, eq - float64(floored)}
	}

	// Distribute remaining units to players with largest fractional parts.
	leftover := totalPrizes - allocated
	sort.Slice(fracs, func(a, b int) bool {
		return fracs[a].frac > fracs[b].frac
	})
	for i := 0; i < leftover && i < n; i++ {
		result[fracs[i].index]++
	}

	return result, nil
}
