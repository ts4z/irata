package state

import (
	"testing"

	"github.com/ts4z/irata/defaults"
)

func TestSumTo10000(t *testing.T) {
	for i, row := range defaults.BARGEPaytable().Rows {
		sum := 0
		for _, p := range row.Percentages {
			sum += p
		}
		if sum != 10000 {
			t.Errorf("PayoutRow %d[%d,%d] sums to %d, want 10000", i, row.MinPlayers, row.MaxPlayers, sum)
		}
	}
}

func TestPayout(t *testing.T) {
	tests := []struct {
		name          string
		prizePool     int64
		numPlayers    int
		wantNumPrizes int
	}{
		{
			name:          "2 players - winner takes all",
			prizePool:     999983,
			numPlayers:    2,
			wantNumPrizes: 1,
		},
		{
			name:          "5 players - top 2 (BARGE 5-8)",
			prizePool:     999983,
			numPlayers:    5,
			wantNumPrizes: 2,
		},
		{
			name:          "10 players - top 3 (BARGE 9-15)",
			prizePool:     999983,
			numPlayers:    10,
			wantNumPrizes: 3,
		},
		{
			name:          "20 players - top 4 (BARGE 16-24)",
			prizePool:     999983,
			numPlayers:    20,
			wantNumPrizes: 4,
		},
		{
			name:          "50 players - top 7 (BARGE 48-55)",
			prizePool:     999983,
			numPlayers:    50,
			wantNumPrizes: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prizes, err := defaults.BARGEPaytable().Payout(tt.prizePool, tt.numPlayers)
			if err != nil {
				t.Fatalf("Payout() returned error: %v", err)
			}

			if len(prizes) != tt.wantNumPrizes {
				t.Errorf("got %d prizes, want %d", len(prizes), tt.wantNumPrizes)
			}

			// Verify total equals prize pool
			total := int64(0)
			for _, p := range prizes {
				total += p
			}
			if total != tt.prizePool {
				t.Errorf("total prizes = %d, want %d", total, tt.prizePool)
			}

			// Verify prizes are in descending order
			for i := 1; i < len(prizes); i++ {
				if prizes[i] > prizes[i-1] {
					t.Errorf("prize[%d] = %d > prize[%d] = %d (should be descending)",
						i, prizes[i], i-1, prizes[i-1])
				}
			}

			// Log the distribution for manual verification
			t.Logf("Prize pool: %d, Players: %d", tt.prizePool, tt.numPlayers)
			for i, prize := range prizes {
				percentage := float64(prize) / float64(tt.prizePool) * 100
				t.Logf("  Place %d: %d (%.2f%%)", i+1, prize, percentage)
			}
		})
	}
}

func TestPayoutZeroPlayers(t *testing.T) {
	prizes, err := defaults.BARGEPaytable().Payout(1_000_000, 0)
	if err == nil {
		t.Fatalf("expected error for 0 players, got nil")
	}
	if len(prizes) != 0 {
		t.Errorf("expected empty slice for 0 players, got %d prizes", len(prizes))
	}
}

func TestPayoutSmallAmounts(t *testing.T) {
	// Test with small prize pool to ensure rounding works
	prizes, err := defaults.BARGEPaytable().Payout(100, 5)
	if err != nil {
		t.Fatalf("Payout() returned error: %v", err)
	}

	total := int64(0)
	for _, p := range prizes {
		total += p
	}

	if total != 100 {
		t.Errorf("total = %d, want 100 (rounding should preserve full pool)", total)
	}
}
