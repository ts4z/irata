package storage

import (
	"github.com/ts4z/irata/ick"
	"github.com/ts4z/irata/model"
)

func makeLevel(desc string) *model.Level {
	return &model.Level{
		Description:     desc,
		DurationMinutes: 30,
		IsBreak:         false,
	}
}

func makeBreak(desc string, durationMins int) *model.Level {
	return &model.Level{
		Description:     desc,
		DurationMinutes: durationMins,
		IsBreak:         true,
	}
}

var tournament *model.Tournament

func init() {
	tournament = makeFakeTournament()
	tournament.FillInitialLevelRemaining()
}

func makeFakeTournament() *model.Tournament {
	// Not implemented, return dummy
	m := &model.Tournament{
		EventName:          "MAIN EVENT",
		VenueName:          "PETERBARGE",
		CurrentLevelNumber: 0,
		CurrentPlayers:     50,
		BuyIns:             50,
		Rebuys:             0,
		AddOns:             0,
		ChipsPerBuyIn:      300,
		ChipsPerAddOn:      0,
		Levels: []*model.Level{
			makeBreak("AWAITING START...", 5),
			makeLevel("5-5 + 5 ANTE"),
			makeLevel("5-10 + 10 ANTE"),
			makeLevel("10-15 + 15 ANTE"),
			makeLevel("10-20 + 20 ANTE"),
			makeBreak("THERE IS BUT ONE RESTROOM. GOOD LUCK.", 1),
			makeLevel("20-40 + 40 ANTE"),
			makeLevel("35-70 + 70 ANTE"),
			makeLevel("60-120 + 120 ANTE"),
			makeBreak("REMOVE $5", 10),
		},
		PrizePool: `1.....$1,650
2.....$1,000
3.......$650
4.......$450
5.......$350
6.......$300
7.......$250
8.......$200
9.......$150
`,
		FooterPlugs: ick.NShuffle([]string{
			`"There are no strangers here, just friends you haven't met yet." <nobr>-Peter Secor</nobr>`,
			"<nobr>THANK YOU MARIO!</nobr> <br><br> <nobr>BUT OUR PRINCESS</nobr> <nobr>IS IN ANOTHER CASTLE!</nobr>",
			"I am a lucky player; a powerful winning force surrounds me. <nobr>-Mike Caro</nobr>",
			"this space intentionally left blank",
			"SPONSORED BY PINBALLPIRATE.COM",
			"SPONSORED BY TS4Z.NET",
			"WWW.BARGE.ORG",
			"this space for rent",
			`"COCKTAILS!"`,
			"WABOR",
			"VISIT THE MAYFAIR CLUB, NEW YORK",
			"May the flop be with you. <nobr>-Doyle Brunson</nobr>",
			"Don't you know who *I* am? <nobr>-Phil Gordon</nobr>",
			"WHO BUT W.B. MASON?",
			`It is morally wrong to allow suckers to keep their money. <nobr>-"Canada Bill" Jones</nobr>`,
			"May all your cards be live and all your pots be monsters. <nobr>-Mike Sexton</nobr>",
			"MAKE SEVEN <br> UP YOURS",
			`"Daddy, I got cider in my ear" -Sky Masterson, in Guys and Dolls`,
			"Trust everyone, but always cut the cards. <nobr>-Benny Binion</nobr>",
			"Poker is a hard way to make an easy living. <nobr>-Doyle Brunson</nobr>",
			"The object of poker is to keep your money away from Phil Ivey for as long as possible. <nobr>-Gus Hansen</nobr>",
			"To be a poker champion, you must have a strong bladder. <nobr>-Jack McClelland</nobr>",
			"No-limit hold’em: Hours of boredom followed by moments of sheer terror. <nobr>-Tom McEvoy</nobr>",
			"Please don't tap on the aquarium.",
			"The rule is this: you spot a man's tell, you don't say a fucking word. <nobr>-Mike McDermott, in Rounders</nobr>",
			`A Smith & Wesson beats four aces. <nobr>-"Canada Bill" Jones</nobr>`,
			"Pay that man his money. <nobr>-Teddy KGB, in Rounders</nobr>",
			"You win some, you lose some, and you keep it to yourself. <nobr>-Mike Caro</nobr>",
			"If you speak the truth, you spoil the game. <nobr>-Mike Caro</nobr>",
			"In the beginning, everything was even money. <nobr>-Mike Caro</nobr>",
			"It's hard to convince a winner that he's losing. <nobr>-Mike Caro</nobr>",
			"If an opponent won't watch you bet, then you probably shouldn't. <nobr>-Mike Caro</nobr>",

			// extractred from QB trip reports
			`<nobr>I toss a chip to the dealer.</nobr> <nobr>Dealer: "What</nobr>'s this for?"
      <nobr>Me: "You</nobr> laughed at my dumb joke."  <nobr>Dealer: "Appreciate it."</nobr> <nobr>-QB</nobr>`,
			`Gillian: "So Dan, how does this work?" Deadhead: "Dan puts out chips.  People take 'em." <nobr>-as reported by QB</nobr>`,
			"Here's the thing about poker... nobody gives a shit. -Dan Goldman",
			"It cost me a couple million dollars to develop this reputation. <nobr>-Daniel Negreanu,</nobr> on being known to be hard-to-bluff",
			`<nobr>"But it's a great game!"</nobr> <nobr>"Yeah, it's a</nobr> great game because YOU'RE in it!" <nobr>-Daniel Negreanu,</nobr>`,
		}),
	}

	m.FillTransients()

	m.IsClockRunning = false

	ick.NShuffle(m.FooterPlugs)

	return m
}

func FetchTournament(id int) (*model.Tournament, error) {
	tournament.FillTransients()
	return tournament, nil
}
