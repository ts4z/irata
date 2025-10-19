package ocsv

/*
This timer took a lot of inspiration from my memories of Patrick Milligan's
Oakleaf Timer.  One thing about the Oakleaf timer is that it manages its
data in CSV format.  For convenience, we can import/export data in a format
that is _somewhat_ Oakleaf compatible.

The Oakleaf format, from its documentation, reads like this:

B, 5,run, Green, silent, , , GAME,STUD, , , , , SEATING,In Progress
R,20,pause,Green, 3chimes,ROUND,1, GAME,STUD,BUTTON,15, BRING IN,5, LIMITS,15-30
R,20,run, Brown, 3chimes,ROUND,2, GAME,STUD,ANTE,5, BRING IN,10, LIMITS,25-50
R,20,run, Green, 3chimes,ROUND,3, GAME,STUD,ANTE,10, BRING IN,20, LIMITS,40-80
B,10,run, Brown, 3chimes,1st, BREAK,GAME,STUD,FINAL,RE-BUYS
R,15,pause,Brown, 3chimes,ROUND,4, GAME,STUD,ANTE,15, BRING IN,25, LIMITS,100-200
R,15,run, Green, 3chimes,ROUND,5, GAME,STUD,ANTE,25, BRING IN,50, LIMITS,150-300
R,15,run, Brown, 3chimes,ROUND,6, GAME,STUD,ANTE,50, BRING IN,75, LIMITS,200-400
R,15,run, Green, 3chimes,ROUND,7, GAME,STUD,ANTE,50, BRING IN,100, LIMITS,300-600

Column 1: Round type:
B Break
R Round
Column 2: Time in minutes. Special case: Zero minutes is an infinite time.
Column 3: State of timer:
Pause Run Hide Pause the timer at the start of this round.
Start the timer running at the start of this round.
Hide and start the timer.
Column 4: Background screen choice. This also matches the deck color. Backgrounds for
Red & Blue as well as Green & Brown KEM decks are provided. In addition,
there are Yellow and Purple background screens which can be used for breaks,
if desired.
Column 5: Sound to play at the start of this round. Current choices are:
Silent Play no sound
1Chime One chime
3Chimes Three chimes
Alarm Smoke alarm sound
Bell_01 Fast bell sound
Bell_02 Slow bell sound
Bell_03 Ships bell sound
Crystal Crystal gong sound
Important note on Sounds: The above list is subject to change, as better
sounds are found. If you like (or dislike) some of the sounds on this list,
please us know so we can keep the “good” sounds and eliminate that “bad”
ones.
There are five areas on the screen that are set from the data file. Each area has a label and data
associated with that label.
Columns 6 & 7: Label and Data for Area 1: Usually the Round or Level
Columns 8 & 9: Label and Data for Area 2: Game choice
Columns 10 & 11: Label and Data for Area 3: Blinds or Antes
Columns 12 & 13: Label and Data for Area 4: Bring-in for Stud, Unused for Flop.
Columns 14 & 15: Label and Data for Area 5: Limits or Blinds (NL)
The text areas are sized to display their contents as large as possible. The contents of the areas
was based on many different tournament structures including TEARS and the World Series of
Poker events. Areas 1 and 4 are “narrow” and Areas 2, 3, and 5 are “wide.”

Unfortunately, I didn't get the display areas to be "the same",
so some alteration is necessary.  On import, the Oakleaf format
will be changed into the model.Structure format somewhat destructively.

*/
