package storage

import (
	"github.com/ts4z/irata/ick"
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/textutil"
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
		EventID:            1,
		EventName:          "PETERBARGE - MAIN EVENT",
		CurrentLevelNumber: 0,
		CurrentPlayers:     50,
		BuyIns:             50,
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
			makeBreak("RACE OFF T5 CHIPS", 10),
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
			`"There are no strangers here, just friends you haven't met yet."
-Peter Secor`,
			"THANK YOU MARIO!\nBUT OUR PRINCESS\n IS IN ANOTHER CASTLE!",
			"I am a lucky player;\na powerful winning force\nsurrounds me.\n-Mike Caro",
			"this space intentionally left blank",
			"SPONSORED BY PINBALLPIRATE.COM",
			"SPONSORED BY TS4Z.NET",
			"WWW.BARGE.ORG",
			"this space for rent",
			`"COCKTAILS!"`,
			"WABOR",
			"VISIT THE MAYFAIR CLUB, NEW YORK",
			"May the flop be with you.\n-Doyle Brunson",
			"Don't you know who *I* am?\n-Phil Gordon",
			"WHO BUT W.B. MASON?",
			`It is morally wrong to allow
suckers to keep their money.
-"Canada Bill" Jones`,
			"May all your cards be\nlive and all your\npots be monsters.\n-Mike Sexton",
			"MAKE SEVEN - UP YOURS",
			"\"Daddy, I got cider in my ear\"\n-Sky Masterson,\nin Guys and Dolls",
			"Trust everyone, but always cut the cards.\n-Benny Binion",
			"Poker is a hard way to\nmake an easy living.\n-Doyle Brunson",
			"The object of poker is to\nkeep your money away from Phil Ivey for as long as possible.\n-Gus Hansen",
			"To be a poker champion,\nyou must have a strong bladder.\n-Jack McClelland",
			"No-limit hold’em:\nHours of boredom\n followed by moments of sheer terror.\n -Tom McEvoy",
			// this is about the longest one-line you can do
			"Please don't tap on the aquarium.",
			"The rule is this:\nyou spot a\nman's tell, you don't\nsay a fucking word.\n-Mike McDermott, in Rounders",
			`A Smith & Wesson beats four aces.
-"Canada Bill" Jones`,
			"Pay that man his money.\n-Teddy KGB, in Rounders",
			"You win some,\nyou lose some,\nand you keep\nit to yourself.\n-Mike Caro",
			"If you speak the truth, you spoil the game.\n-Mike Caro",
			"In the beginning,\neverything was\neven money.\n-Mike Caro",
			"It's hard to convince\na winner that he's losing.\n-Mike Caro",
			"If an opponent\nwon't watch you bet,\nthen you\nprobably shouldn't.\n-Mike Caro",

			// extractred from QB trip reports
			`I toss a chip to the dealer.
Dealer: "What's this for?"
Me: "You laughed at my dumb joke."
Dealer: "Appreciate it." -QB`,
			`Gillian: "So Dan,
how does this work?"
Deadhead: "Dan puts
out chips.
People take'em."
-as reported by QB`,
			"Here's the thing about poker...\nnobody gives a shit.\n-Dan Goldman",
			"It cost me a couple million dollars\nto develop this reputation.\n-Daniel Negreanu,\non being known to be hard-to-bluff",
			`"But it's a great game!"
"Yeah, it's a great game
because YOU'RE in it!"
-Daniel Negreanu`,
		}),
	}

	m.FillTransients()

	m.IsClockRunning = false

	return m
}

func FetchTournament(id int) (*model.Tournament, error) {
	tournament.FillTransients()
	return tournament, nil
}

func FetchTournamentForView(id int) (*model.Tournament, error) {
	tournament.FillTransients()
	for i, plug := range tournament.FooterPlugs {
		tournament.FooterPlugs[i] = textutil.WrapLinesInNOBR(plug)
	}
	return tournament, nil
}
