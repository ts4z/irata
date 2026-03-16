package icm

import (
	"math"
	"testing"
)

func TestTwoPlayersEqualChips(t *testing.T) {
	chips := []int{1000, 1000}
	prizes := []int{60, 40}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Equal chips → equal equity → each gets (60+40)/2 = 50
	assertEquity(t, eq, 0, 50.0)
	assertEquity(t, eq, 1, 50.0)
}

func TestTwoPlayersUnequalChips(t *testing.T) {
	chips := []int{3000, 1000}
	prizes := []int{60, 40}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// P(0 wins) = 0.75, P(1 wins) = 0.25
	// EV(0) = 0.75*60 + 0.25*40 = 55
	// EV(1) = 0.25*60 + 0.75*40 = 45
	assertEquity(t, eq, 0, 55.0)
	assertEquity(t, eq, 1, 45.0)
}

func TestThreePlayersEqualChips(t *testing.T) {
	chips := []int{1000, 1000, 1000}
	prizes := []int{50, 30, 20}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Equal chips → equal equity → 100/3 ≈ 33.33
	expected := 100.0 / 3.0
	for i := range eq {
		assertEquity(t, eq, i, expected)
	}
}

func TestThreePlayersUnequalChips(t *testing.T) {
	chips := []int{5000, 3000, 2000}
	prizes := []int{50, 30, 20}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	// Manually computed Malmuth-Harville values:
	// EV(0) ≈ 38.393, EV(1) ≈ 32.750, EV(2) ≈ 28.857
	assertEquity(t, eq, 0, 38.393)
	assertEquity(t, eq, 1, 32.750)
	assertEquity(t, eq, 2, 28.857)

	assertSumEquals(t, eq, 100.0)
}

func TestChipLeaderGetsLessThanFirstPrize(t *testing.T) {
	// A key ICM property: even the chip leader's equity is less than
	// 1st place prize, because they might not win.
	chips := []int{8000, 1000, 1000}
	prizes := []int{100, 50, 20}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	if eq[0] >= float64(prizes[0]) {
		t.Errorf("chip leader equity %.2f should be less than 1st prize %d", eq[0], prizes[0])
	}

	assertSumEquals(t, eq, 170.0)
}

func TestShortStackGetsMoreThanLastPrize(t *testing.T) {
	// Another ICM property: even the short stack's equity exceeds the
	// last-place prize, because they might finish higher.
	chips := []int{8000, 1000, 1000}
	prizes := []int{100, 50, 20}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	lastPrize := float64(prizes[len(prizes)-1])
	if eq[2] <= lastPrize {
		t.Errorf("short stack equity %.2f should exceed last prize %.0f", eq[2], lastPrize)
	}
}

func TestFewerPrizesThanPlayers(t *testing.T) {
	// 4 players but only 2 prizes; positions 3-4 get $0.
	chips := []int{4000, 3000, 2000, 1000}
	prizes := []int{70, 30}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertSumEquals(t, eq, 100.0)

	// More chips → more equity
	for i := 0; i < len(eq)-1; i++ {
		if eq[i] <= eq[i+1] {
			t.Errorf("equity should decrease with chips: eq[%d]=%.2f <= eq[%d]=%.2f",
				i, eq[i], i+1, eq[i+1])
		}
	}
}

func TestOnePlayer(t *testing.T) {
	chips := []int{5000}
	prizes := []int{100}

	eq, err := CalculateEquity(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	assertEquity(t, eq, 0, 100.0)
}

func TestChopRounding(t *testing.T) {
	chips := []int{1000, 1000, 1000}
	prizes := []int{50, 30, 20}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	sum := 0
	for _, c := range chopped {
		sum += c
	}
	if sum != 100 {
		t.Errorf("sum of chopped amounts %d != 100", sum)
	}

	// Equal chips → all should get 33 or 34 (100/3 rounded)
	for i, c := range chopped {
		if c != 33 && c != 34 {
			t.Errorf("chopped[%d] = %d, expected 33 or 34", i, c)
		}
	}
}

func TestChopPreservesTotal(t *testing.T) {
	chips := []int{5000, 3000, 2000}
	prizes := []int{1000, 600, 400}

	chopped, err := Chop(chips, prizes)
	if err != nil {
		t.Fatal(err)
	}

	sum := 0
	for _, c := range chopped {
		sum += c
	}
	if sum != 2000 {
		t.Errorf("sum of chopped amounts %d != 2000", sum)
	}
}

func TestErrorNoPlayers(t *testing.T) {
	_, err := CalculateEquity(nil, nil)
	if err == nil {
		t.Error("expected error for nil chips")
	}
}

func TestErrorZeroChips(t *testing.T) {
	_, err := CalculateEquity([]int{100, 0}, []int{60, 40})
	if err == nil {
		t.Error("expected error for zero chips")
	}
}

func TestErrorNegativeChips(t *testing.T) {
	_, err := CalculateEquity([]int{100, -50}, []int{60, 40})
	if err == nil {
		t.Error("expected error for negative chips")
	}
}

func assertEquity(t *testing.T, eq []float64, player int, expected float64) {
	t.Helper()
	if math.Abs(eq[player]-expected) > 0.01 {
		t.Errorf("player %d: expected %.3f, got %.3f", player, expected, eq[player])
	}
}

func assertSumEquals(t *testing.T, eq []float64, expected float64) {
	t.Helper()
	sum := 0.0
	for _, e := range eq {
		sum += e
	}
	if math.Abs(sum-expected) > 0.01 {
		t.Errorf("sum of equities %.3f != %.3f", sum, expected)
	}
}
