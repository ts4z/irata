package proportional

import (
	"math"
)

type Chopper struct{}

func (c *Chopper) Name() string {
	return "Proportional"
}

func (c *Chopper) Chop(chips []int, prizes []int) ([]int, error) {
	return Chop(chips, prizes)
}

func (c *Chopper) MaxPlayers() int {
	return math.MaxInt
}

func Chop(chips []int, prizes []int) ([]int, error) {
	if len(chips) == 0 {
		return nil, nil
	}

	// Pad or truncate prizes to match the number of players.
	padded := make([]int, len(chips))
	for i := 0; i < len(chips) && i < len(prizes); i++ {
		padded[i] = prizes[i]
	}

	totalChips := 0
	for _, c := range chips {
		totalChips += c
	}

	totalPrizePool := 0
	for _, p := range padded {
		totalPrizePool += p
	}

	chopPrizePool := totalPrizePool
	floorPrize := 0
	if len(chips) <= len(prizes) {
		// everybody gets nth place money
		floorPrize = padded[len(chips)-1]
		chopPrizePool -= floorPrize * len(chips)
	}

	totalAllocated := 0
	tpf := float64(chopPrizePool)
	choppedPrizes := make([]int, len(chips))
	for i := range chips {
		choppedPrizes[i] = floorPrize + int((float64(chips[i])/float64(totalChips))*tpf)
		totalAllocated += choppedPrizes[i]
	}

	{
		i := 0
		remainder := totalPrizePool - totalAllocated
		for remainder > 0 {
			choppedPrizes[i]++
			remainder--
			i++
			if i >= len(choppedPrizes) {
				i = 0
			}
		}
	}

	return choppedPrizes, nil
}
