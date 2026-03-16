package proportional

import (
	"testing"
)

func TestTwoPlayersEqualChips(t *testing.T) {
	chips := []int{1000, 1000}
	prizes := []int{60, 40}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Equal chips → equal share → each gets 50
	assertChop(t, chopped, 0, 50)
	assertChop(t, chopped, 1, 50)
	assertSumEquals(t, chopped, 100)
}

func TestTwoPlayersUnequalChips(t *testing.T) {
	chips := []int{3000, 1000}
	prizes := []int{60, 40}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Floor prize = 40 each, remaining 20 split 3:1 → 15 + 5
	// P0 = 40 + 15 = 55, P1 = 40 + 5 = 45
	assertChop(t, chopped, 0, 55)
	assertChop(t, chopped, 1, 45)
	assertSumEquals(t, chopped, 100)
}

func TestThreePlayersEqualChips(t *testing.T) {
	chips := []int{1000, 1000, 1000}
	prizes := []int{50, 30, 20}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Equal chips → 100/3 ≈ 33.33 → two get 33, one gets 34
	for i, c := range chopped {
		if c != 33 && c != 34 {
			t.Errorf("chopped[%d] = %d, expected 33 or 34", i, c)
		}
	}
	assertSumEquals(t, chopped, 100)
}

func TestThreePlayersUnequalChips(t *testing.T) {
	chips := []int{5000, 3000, 2000}
	prizes := []int{50, 30, 20}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Floor prize = 20 each, remaining 40 split 5:3:2
	// P0 = 20 + 20 = 40, P1 = 20 + 12 = 32, P2 = 20 + 8 = 28
	assertChop(t, chopped, 0, 40)
	assertChop(t, chopped, 1, 32)
	assertChop(t, chopped, 2, 28)
	assertSumEquals(t, chopped, 100)
}

func TestChipLeaderGetsMoreWithProportional(t *testing.T) {
	// Unlike ICM, proportional can give the chip leader MORE than 1st prize
	// since it's purely chip-proportional.
	chips := []int{8000, 1000, 1000}
	prizes := []int{100, 50, 20}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Chip leader has 80% of chips — they should get the biggest share
	if chopped[0] <= chopped[1] || chopped[0] <= chopped[2] {
		t.Errorf("chip leader chop %d should be largest", chopped[0])
	}
	assertSumEquals(t, chopped, 170)
}

func TestShortStackGetsMoreThanLastPrize(t *testing.T) {
	chips := []int{8000, 1000, 1000}
	prizes := []int{100, 50, 20}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	if chopped[2] <= prizes[2] {
		t.Errorf("short stack chop %d should exceed last prize %d", chopped[2], prizes[2])
	}
}

func TestFewerPrizesThanPlayers(t *testing.T) {
	// 4 players but only 2 prizes; no floor prize since more players than paid positions.
	chips := []int{4000, 3000, 2000, 1000}
	prizes := []int{70, 30}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertSumEquals(t, chopped, 100)

	// More chips → more chop
	for i := 0; i < len(chopped)-1; i++ {
		if chopped[i] < chopped[i+1] {
			t.Errorf("chop should decrease with chips: chopped[%d]=%d < chopped[%d]=%d",
				i, chopped[i], i+1, chopped[i+1])
		}
	}
}

func TestOnePlayer(t *testing.T) {
	chips := []int{5000}
	prizes := []int{100}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertChop(t, chopped, 0, 100)
}

func TestNoPlayers(t *testing.T) {
	chopped, err := Chop(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if chopped != nil {
		t.Errorf("expected nil for no players, got %v", chopped)
	}
}

func TestPreservesTotal(t *testing.T) {
	chips := []int{5000, 3000, 2000}
	prizes := []int{1000, 600, 400}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertSumEquals(t, chopped, 2000)
}

func TestMorePlayersThanPrizesProportional(t *testing.T) {
	// 5 players, 3 prizes — all prize money split proportionally by chips
	chips := []int{5000, 4000, 3000, 2000, 1000}
	prizes := []int{500, 300, 200}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertSumEquals(t, chopped, 1000)

	// Everyone should get something since they all have chips
	for i, c := range chopped {
		if c <= 0 {
			t.Errorf("chopped[%d] = %d, should be > 0", i, c)
		}
	}
}

func TestEqualChipsEqualPrizes(t *testing.T) {
	// All equal chips, equal prizes → everyone gets the same
	chips := []int{1000, 1000, 1000, 1000}
	prizes := []int{100, 100, 100, 100}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	for i, c := range chopped {
		assertChop(t, chopped, i, 100)
		_ = c
	}
	assertSumEquals(t, chopped, 400)
}

func TestChopperInterface(t *testing.T) {
	c := &Chopper{}
	chips := []int{3000, 1000}
	prizes := []int{60, 40}

	chopped, err := c.Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertSumEquals(t, chopped, 100)

	if c.MaxPlayers() < 1000000 {
		t.Errorf("MaxPlayers should be very large, got %d", c.MaxPlayers())
	}
}

func assertChop(t *testing.T, chopped []int, player int, expected int) {
	t.Helper()
	if chopped[player] != expected {
		t.Errorf("player %d: expected %d, got %d", player, expected, chopped[player])
	}
}

func assertSumEquals(t *testing.T, chopped []int, expected int) {
	t.Helper()
	sum := 0
	for _, c := range chopped {
		sum += c
	}
	if sum != expected {
		t.Errorf("sum of chopped amounts %d != %d", sum, expected)
	}
}
