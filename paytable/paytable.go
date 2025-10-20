package paytable

// Package paytable provides data models and stateless functions for representing
// payout pay tables.

import (
	"fmt"
)

// Row defines the payout percentages for a range of player counts.
// Percentages are in basis points (10000 = 100%).
type Row struct {
	MinPlayers  int   // Minimum number of players (inclusive)
	MaxPlayers  int   // Maximum number of players (inclusive)
	Percentages []int // Percentages in basis points, index 0 = 1st place
}

// Paytable is a collection of payout percentages that define prize distributions
// for different player count ranges.
type Paytable struct {
	ID   int64  // Unique identifier for the payout table
	Name string // Name of the payout table (e.g., "BARGE 2025")
	Rows []Row  // Ordered list of payout rows
}

// PaytableSlug is a lightweight representation of a payout table for lists.
type PaytableSlug struct {
	Name string
	ID   int64
}

// Payout calculates prize distribution for this specific payout table.
func (pt *Paytable) Payout(totalPrizePool int64, numPlayers int) ([]int64, error) {
	// Get the payout percentages for the given number of players
	percentages := pt.findRow(numPlayers)
	if len(percentages) == 0 {
		return nil, fmt.Errorf("no payout row found for %d players", numPlayers)
	}

	prizes := make([]int64, len(percentages))
	totalAllocated := int64(0)

	for i := 0; i < len(percentages); i++ {
		prizes[i] = (totalPrizePool * int64(percentages[i])) / 10000
		totalAllocated += prizes[i]
	}

	remainder := totalPrizePool - totalAllocated
	for i := 0; remainder > 0; {
		prizes[i%len(prizes)]++
		i++
		remainder--
	}

	return prizes, nil
}

// FindRow finds the paytable row for the number of players, or nil if there isn't one.
func (pt *Paytable) findRow(numPlayers int) []int {
	for _, row := range pt.Rows {
		if numPlayers >= row.MinPlayers && numPlayers <= row.MaxPlayers {
			return row.Percentages
		}
	}
	return nil
}

func (pt *Paytable) Clone() *Paytable {
	clone := &Paytable{
		ID:   pt.ID,
		Name: pt.Name,
		Rows: make([]Row, len(pt.Rows)),
	}
	for i, row := range pt.Rows {
		clone.Rows[i] = row
		clone.Rows[i].Percentages = make([]int, len(row.Percentages))
		copy(clone.Rows[i].Percentages, row.Percentages)
	}
	return clone
}
