DROP TABLE structures CASCADE;
DROP TABLE tournaments CASCADE;
DROP TABLE text_footer_plugs CASCADE;
DROP TABLE footer_plug_sets CASCADE;
DROP TABLE site_info CASCADE;
DROP TABLE users CASCADE;
DROP TABLE passwords CASCADE;
DROP TABLE user_email_addresses CASCADE;

CREATE TABLE users (
    user_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nick VARCHAR(20) NOT NULL UNIQUE,
    is_admin BOOLEAN DEFAULT FALSE NOT NULL
);

CREATE INDEX idx_user_nick ON users(nick);

CREATE TABLE passwords (
    password_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    hashed_password VARCHAR(255) NOT NULL,
    expires TIMESTAMP WITHOUT TIME ZONE
);

CREATE TABLE user_email_addresses (
    email_address VARCHAR(255) NOT NULL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE INDEX idx_user_email_addresses_email 
    ON user_email_addresses(email_address);

CREATE TABLE footer_plug_sets (
   id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
   name TEXT NOT NULL,
   version BIGINT DEFAULT 0 NOT NULL
);

CREATE TABLE text_footer_plugs (
   id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
   version BIGINT DEFAULT 0 NOT NULL,
   footer_plug_set_id BIGINT NOT NULL REFERENCES footer_plug_sets(id) ON DELETE CASCADE,
   text TEXT NOT NuLL
);

CREATE TABLE site_info (
   key TEXT PRIMARY KEY UNIQUE NOT NULL,
   value JSONB NOT NULL,
   version BIGINT DEFAULT 0 NOT NULL
);

INSERT INTO site_info (key, value) VALUES
  ('conf', $json$
    {
      "Name": "Irata Poker Tournament Clock",
      "Site": "iratapoker.com",
      "Theme": "irata"
    }
  $json$);
  
CREATE TABLE tournaments (
       tournament_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
       handle VARCHAR(30) UNIQUE NOT NULL,
       version BIGINT DEFAULT 0 NOT NULL,
       model_data JSONB NOT NULL
);

CREATE INDEX idx_tournaments_handle 
    ON tournaments(handle); 

CREATE TABLE structures (
       structure_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
       version BIGINT DEFAULT 0,
       name TEXT NOT NULL,
       model_data JSONB NOT NULL
);

INSERT INTO tournaments (tournament_id, handle, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (1, 'peterbarge', $json$
    {
       "EventName": "PeterBARGE",
       "Description": "$100 Freezeout at Pinball Pirate",
       "FooterPlugsID": 1,
       "Structure": {
       "ChipsPerBuyIn": 3000,
       "ChipsPerAddOn": 0,
           "Levels":   [
           { "Banner": "PeterBARGE 3D", "Description": "SU & DEAL @ 11:00AM", "DurationMinutes": 59, "IsBreak": true },
           { "Banner": "LEVEL 1", "Description": "BLINDS 25-50", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 2", "Description": "BLINDS 50-75", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 3", "Description": "BLINDS 50-100", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 4", "Description": "BLINDS 75-150", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 5", "Description": "BLINDS 100-200", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "BREAK", "Description": "PICTURE TIME & REMOVE 25s", "DurationMinutes": 20, "IsBreak": true },
           { "Banner": "LEVEL 6", "Description": "BLINDS 200-300", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 7", "Description": "BLINDS 200-400", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 8", "Description": "BLINDS 300-600", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 9", "Description": "BLINDS 500-1000", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 10", "Description": "BLINDS 800-1600", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "BREAK", "Description": "REMOVE 100s", "DurationMinutes": 5, "IsBreak": true },
           { "Banner": "LEVEL 11", "Description": "BLINDS 1500-2500", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 12", "Description": "BLINDS 2K-4K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 13?!", "Description": "BLINDS 3K-6K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 14?!!", "Description": "BLINDS 5K-10K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 15?!?!!", "Description": "BLINDS 6K-12K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 16!!????", "Description": "BLINDS 8K-16K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 17 (sigh)", "Description": "BLINDS 10K-20K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "GO HOME ALREADY", "Description": "BLINDS 55000-55000", "DurationMinutes": 59, "IsBreak": false }
       ]},
       "State": {
           "IsClockRunning": false,
           "CurrentLevelNumber": 0,
           "TimeRemainingMillis": 3599000,
           "CurrentPlayers": 33,
           "BuyIns": 33,
           "PrizePool": "Yadda\nYadda\nYadda"
       }
    }
    $json$);

INSERT INTO tournaments (tournament_id, handle, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (2, 'main', $json$
    {
       "EventName": "WSOP #61 MAIN EVENT",
       "Description": "The Big Dance",
       "FooterPlugsID": 1,
       "Structure": {
       "ChipsPerBuyIn": 60000,
       "ChipsPerAddOn": 0,
           "Levels":   [
           { "Banner": "SETTING UP", "Description": "PLAYERS: TAKE YOUR SEATS", "DurationMinutes": 60, "IsBreak": true },
           { "Banner": "LEVEL 1 - NO LIMIT TEXAS HOLDEM", "Description": "BLINDS 100-100 w/100 BB ANTE", "DurationMinutes": 120 },
           { "Banner": "LEVEL 2 - NO LIMIT TEXAS HOLDEM", "Description": "BLINDS 100-200 w/200 BB ANTE", "DurationMinutes": 120 },
           { "Banner": "LEVEL 3 - NO LIMIT TEXAS HOLDEM", "Description": "BLINDS 200-300 w/300 BB ANTE", "DurationMinutes": 120 },
           { "Banner": "LEVEL 3 - NO LIMIT TEXAS HOLDEM", "Description": "BLINDS 200-400 w/400 BB ANTE", "DurationMinutes": 120 }
       ]},
       "State": {
           "IsClockRunning": false,
           "CurrentLevelNumber": 0,
           "TimeRemainingMillis": 3599000,
           "CurrentPlayers": 33,
           "BuyIns": 33,
           "TotalChips": 9900,
           "PrizePool": "1..$10,000,000\n2...$5,000,000\n3...$3,000,000\n......"
       }
    }
    $json$);


INSERT INTO structures (structure_id, name, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (1, 'BREMER 3000', $json$
    {
       "Levels": [
           {
              "Banner": "WELCOME TO THE EVENT",
              "Description": "AWAITING START...",
              "DurationMinutes": 60,
              "IsBreak": true
           },
           {
              "Banner": "LEVEL 1",
              "Description": "25-50 + 50 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           },
           {
              "Banner": "LEVEL 2",
              "Description": "50-75 + 75 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           },
           {
              "Banner": "LEVEL 3",
              "Description": "50-100 + 100 ANTE",
              "Durationminutes": 18,
              "IsBreak": false
           },
           {
              "Banner": "LEVEL 4",
              "Description": "75-150 + 150 ANTE",
              "Durationminutes": 18,
              "IsBreak": false
           },
           {
              "Banner": "LEVEL 5",
              "Description": "100-200 + 200 ANTE",
              "Durationminutes": 18,
              "IsBreak": false
           },
           {
              "Banner": "LEVEL 6",
              "Description": "PICTURE TIME",
              "DurationMinutes": 20,
              "IsBreak": true
           },
           {
              "Banner": "LEVEL 7",
              "Description": "200-300 + 300 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           },
           {
              "Banner": "LEVEL 8",
              "Description": "200-400 + 400 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           }
       ]
    }
    $json$ );

-- Insert a default plug set
INSERT INTO footer_plug_sets (id, name) OVERRIDING SYSTEM VALUE VALUES (1, 'Mostly BARGE In-Jokes') ;

-- Insert plugs (each plug as a separate row)
INSERT INTO text_footer_plugs (footer_plug_set_id, text) VALUES
(1, '"There are no strangers here,
just friends
you haven''t met yet."
-Peter Secor'),
(1, 'THANK YOU MARIO!
BUT OUR PRINCESS
 IS IN ANOTHER CASTLE!'),
(1, 'I am a lucky player;
a powerful winning force
surrounds me.
-Mike Caro'),
(1, 'this space intentionally left blank'),
(1, 'SPONSORED BY PINBALLPIRATE.COM'),
(1, 'SPONSORED BY TS4Z.NET'),
(1, 'NOT SPONSORED BY
POKERSTARS.COM'),
(1, 'WWW.BARGE.ORG'),
(1, 'WWW.BJRGE.ORG'),
(1, 'FARGOPOKER.ORG'),
(1, 'ATLARGEPOKER.COM'),
(1, 'ARGEMPOKER.COM'),
(1, 'PETER.BARGE.ORG'),
(1, 'CRAFTPOKER.COM'),
(1, 'BARGECHIPS.ORG'),
(1, 'this space for rent'),
(1, '"COCKTAILS!"'),
(1, 'WABOR'),
(1, 'WHEN IN NEW YORK...
VISIT THE MAYFAIR CLUB'),
(1, 'WHEN IN PARIS...
VISIT THE AVIATION CLUB'),
(1, 'May the flop be with you.
-Doyle Brunson'),
(1, 'Don''t you know who **I** am?
-Phil Gordon'),
(1, 'WHO BUT W.B. MASON?'),
(1, 'It is morally wrong to allow
suckers to keep their money.
-"Canada Bill" Jones'),
(1, 'May all your cards be
live and all your
pots be monsters.
-Mike Sexton'),
(1, 'MAKE SEVEN - UP YOURS'),
(1, '"Daddy, I got cider in my ear"
-Sky Masterson,
in Guys and Dolls'),
(1, 'Trust everyone,
but always
cut the cards.
-Benny Binion'),
(1, 'Poker is a hard way to
make an easy living.
-Doyle Brunson'),
(1, 'The object of poker is to
keep your money away from
Phil Ivey
for as long as possible.
-Gus Hansen'),
(1, 'To be a poker champion,
you must have a strong bladder.
-Jack McClelland'),
(1, 'No-limit hold’em:
Hours of boredom
 followed by moments of sheer terror.
 -Tom McEvoy'),
(1, 'Please don''t tap on the aquarium.'),
(1, 'The rule is this:
you spot a
man''s tell, you don''t
say a fucking word.
-Mike McDermott, in Rounders'),
(1, 'A Smith & Wesson
beats four aces.
-"Canada Bill" Jones'),
(1, 'Pay that man his money.
-Teddy KGB, in Rounders'),
(1, 'You win some,
you lose some,
and you keep
it to yourself.
-Mike Caro'),
(1, 'If you speak the truth,
you spoil the game.
-Mike Caro'),
(1, 'In the beginning,
everything was
even money.
-Mike Caro'),
(1, 'It''s hard to convince
a winner that he''s losing.
-Mike Caro'),
(1, 'If an opponent
won''t watch you bet,
then you
probably shouldn''t.
-Mike Caro'),
(1, 'Just play every hand,
you can’t miss them all.
-Sammy Farha'),
(1, 'Last night
I stayed
up late playing
poker
with Tarot cards.
I got a full house
and four
people died.
-Steven Wright'),
(1, 'Going on tilt
is not 
"mixing up your play."
-Steve Badger'),
(1, 'The guy who invented
poker was bright,
but the guy who
invented the chip
was a genius.
-"Big Julie" Weintraub'),
(1, 'Sex is good,
they say,
but poker lasts longer.
-Al Alvarez'),
(1, 'Money won
is twice as sweet
as money earned.
-"Fast Eddie" Felson
in The Color of Money'),
(1, 'Fold and live
to fold again. -Stu Ungar'),
(1, 'Life is not
always a matter
of holding
good cards, but
sometimes,
playing a
poor hand
well.
-Jack London'),
(1, 'The lack of money is the
root of all evil.
-Mark Twain'),
(1, 'Learning to
play two pairs
correctly is as difficult
as getting
a college education,
and just as expensive.
-Mark Twain'),
(1, 'You''re not going
to like this,
Nolan.'),
(1, 'I toss a chip to the dealer.
Dealer: "What''s this for?"
Me: "You laughed at my dumb joke."
Dealer: "Appreciate it." -QB'),
(1, 'Gillian: "So Dan,
how does
this work?
"Deadhead: "Dan puts
out chips.
People take ''em."
-as reported by QB'),
(1, 'Here''s the thing about poker...
nobody gives a shit.
-Dan Goldman'),
(1, 'It cost me a couple
million dollars
to develop
this reputation.
-Daniel Negreanu,
on being known to be
hard-to-bluff'),
(1, '"But it''s a great game!"
"Yeah, it''s a great game
because YOU''RE in it!"
-Daniel Negreanu'),
(1, 'This is my third rodeo.');
