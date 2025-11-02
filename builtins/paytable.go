package builtins

import (
	"github.com/ts4z/irata/paytable"
)

// BARGEPayoutTable is the official BARGE 2025 payout structure from page 14
// of the 2025 BARGE Structures PDF.
// Tournaments with less than 5 players are winner take all.
var bargePayoutTable = &paytable.Paytable{
	Name:      "BARGE Unified Poker Payouts",
	Increment: 5,
	Rows: []paytable.Row{
		{
			MinPlayers:  1,
			MaxPlayers:  4,
			Percentages: []int{10000}, // Winner takes all
		},
		{
			MinPlayers:  5,
			MaxPlayers:  8,
			Percentages: []int{6500, 3500},
		},
		{
			MinPlayers:  9,
			MaxPlayers:  15,
			Percentages: []int{5000, 3000, 2000},
		},
		{
			MinPlayers:  16,
			MaxPlayers:  24,
			Percentages: []int{4200, 2600, 1800, 1400},
		},
		{
			MinPlayers:  25,
			MaxPlayers:  35,
			Percentages: []int{3600, 2400, 1700, 1300, 1000},
		},
		{
			MinPlayers:  36,
			MaxPlayers:  47,
			Percentages: []int{3100, 2200, 1700, 1300, 1000, 700},
		},
		{ // 6
			MinPlayers:  48,
			MaxPlayers:  55,
			Percentages: []int{2800, 2100, 1600, 1300, 1000, 700, 500},
		},
		{ // 7
			MinPlayers:  56,
			MaxPlayers:  64,
			Percentages: []int{2700, 2000, 1600, 1200, 900, 700, 500, 400},
		},
		{ // 8
			MinPlayers:  65,
			MaxPlayers:  72,
			Percentages: []int{2600, 1900, 1500, 1200, 900, 700, 500, 400, 300},
		},
		{ // 9
			MinPlayers:  73,
			MaxPlayers:  80,
			Percentages: []int{2500, 1900, 1400, 1100, 900, 700, 500, 400, 300, 300},
		},
		{ // 10
			MinPlayers:  81,
			MaxPlayers:  96,
			Percentages: []int{2500, 1800, 1300, 1000, 800, 600, 500, 400, 300, 300, 250, 250},
		},
		{ // 11
			MinPlayers:  97,
			MaxPlayers:  120,
			Percentages: []int{2500, 1700, 1200, 900, 700, 600, 400, 300, 300, 300, 250, 250, 200, 200, 200},
		},
		{ // 12
			MinPlayers:  121,
			MaxPlayers:  144,
			Percentages: []int{2400, 1600, 1200, 900, 700, 500, 400, 300, 250, 250, 225, 225, 200, 200, 200, 150, 150, 150},
		},
		{
			MinPlayers:  145,
			MaxPlayers:  168,
			Percentages: []int{2300, 1500, 1100, 850, 600, 500, 400, 300, 250, 250, 225, 225, 200, 200, 200, 150, 150, 150, 150, 150, 150},
		},
	},
}

func BARGEPaytable() *paytable.Paytable {
	return bargePayoutTable.Clone()
}
