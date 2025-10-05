package defaults

import (
	"github.com/ts4z/irata/model"
	"github.com/ts4z/irata/textutil"
)

func FooterPlugs() *model.FooterPlugs {
	return &model.FooterPlugs{
		FooterPlugsID: -1,
		TextPlugs:     TextPlugs(),
	}
}

func TextPlugs() []string {
	a := []string{
		`"There are no strangers here,
just friends
you haven't met yet."
-Peter Secor`,
		"THANK YOU MARIO!\nBUT OUR PRINCESS\n IS IN ANOTHER CASTLE!",
		"I am a lucky player;\na powerful winning force\nsurrounds me.\n-Mike Caro",
		"this space intentionally left blank",
		"SPONSORED BY PINBALLPIRATE.COM",
		"SPONSORED BY TS4Z.NET",
		"WWW.BARGE.ORG",
		"WWW.BJRGE.ORG",
		"this space for rent",
		"\"COCKTAILS!\"",
		"WABOR",
		"WHEN IN NEW YORK...\nVISIT THE MAYFAIR CLUB",
		"May the flop be with you.\n-Doyle Brunson",
		"Don't you know who *I* am?\n-Phil Gordon",
		"WHO BUT W.B. MASON?",
		"It is morally wrong to allow\nsuckers to keep their money.\n-\"Canada Bill\" Jones",
		"May all your cards be\nlive and all your\npots be monsters.\n-Mike Sexton",
		"MAKE SEVEN - UP YOURS",
		"\"Daddy, I got cider in my ear\"\n-Sky Masterson,\nin Guys and Dolls",
		"Trust everyone, but always cut the cards.\n-Benny Binion",
		"Poker is a hard way to\nmake an easy living.\n-Doyle Brunson",
		"The object of poker is to\nkeep your money away from\nPhil Ivey\nfor as long as possible.\n-Gus Hansen",
		"To be a poker champion,\nyou must have a strong bladder.\n-Jack McClelland",
		"No-limit hold’em:\nHours of boredom\n followed by moments of sheer terror.\n -Tom McEvoy",
		// this is about the longest one-line you can do
		"Please don't tap on the aquarium.",
		"The rule is this:\nyou spot a\nman's tell, you don't\nsay a fucking word.\n-Mike McDermott, in Rounders",
		"A Smith & Wesson beats four aces.\n-\"Canada Bill\" Jones",
		"Pay that man his money.\n-Teddy KGB, in Rounders",
		"You win some,\nyou lose some,\nand you keep\nit to yourself.\n-Mike Caro",
		"If you speak the truth,\nyou spoil the game.\n-Mike Caro",
		"In the beginning,\neverything was\neven money.\n-Mike Caro",
		"It's hard to convince\na winner that he's losing.\n-Mike Caro",
		"If an opponent\nwon't watch you bet,\nthen you\nprobably shouldn't.\n-Mike Caro",

		// extractred from QB trip reports
		"I toss a chip to the dealer.\nDealer: \"What's this for?\"\nMe: \"You laughed at my dumb joke.\"\nDealer: \"Appreciate it.\" -QB",
		"Gillian: \"So Dan,\nhow doesthis work?\"Deadhead: \"Dan puts\nout chips. People take 'em.\"\n-as reported by QB",
		"Here's the thing about poker...\nnobody gives a shit.\n-Dan Goldman",
		"It cost me a couple million dollars\nto develop this reputation.\n-Daniel Negreanu,\non being known to be hard-to-bluff",
		"\"But it's a great game!\"\n\"Yeah, it's a great game\nbecause YOU'RE in it!\"\n-Daniel Negreanu",
	}

	for i, s := range a {
		a[i] = textutil.WrapLinesInNOBR(s)
	}

	return a
}
