
DROP TABLE structures;
DROP TABLE tournaments;

CREATE TABLE tournaments (
       tournament_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
       optimistic_lock BIGINT DEFAULT 0,
       model_data JSONB NOT NULL
);

CREATE TABLE structures (
       structure_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
       optimistic_lock BIGINT DEFAULT 0,
       model_data JSONB NOT NULL
);

INSERT INTO tournaments (tournament_id, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (1, $json$
    {
       "EventName": "PeterBARGE",
       "Description": "$100 Freezeout at Pinball Pirate",
       "FooterPlugsID": 1,
       "Structure": {
       "ChipsPerBuyIn": 3000,
       "ChipsPerAddOn": 0,
           "Levels":   [
           { "Banner": "SETTING UP", "Description": "PLAYERS: PLEASE TAKE YOUR SEATS", "DurationMinutes": 60, "IsBreak": true },
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
           { "Banner": "LEVEL 11", "Description": "BLINDS 1500-2000", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 12", "Description": "BLINDS 2K-4K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 13?!", "Description": "BLINDS 3K-6K", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 14?!!", "Description": "BLINDS 5K-10K BLINDS", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 15?!?!!", "Description": "BLINDS 6K-12K BLINDS", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 16!!????", "Description": "BLINDS 8K-16K BLINDS", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 17", "Description": "BLINDS 10K-20K BLINDS", "DurationMinutes": 18, "IsBreak": false },
           { "Banner": "LEVEL 18 GO HOME ALREADY", "Description": "BLINDS 55000-55000", "DurationMinutes": 18, "IsBreak": false }
       ]},
       "State": {
           "IsClockRunning": false,
           "CurrentLevelNumber": 0,
           "TimeRemainingMillis": 3599000,
           "CurrentPlayers": 33,
           "BuyIns": 33,
           "TotalChips": 9900,
           "PrizePool": "Yadda\nYadda\nYadda"
       }
    }
    $json$);

INSERT INTO tournaments (tournament_id, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (2, $json$
    {
       "EventName": "WSOP #61 MAIN EVENT",
       "Description": "The Big Dance",
       "FooterPlugsID": 1,
       "Structure": {
       "ChipsPerBuyIn": 60000,
       "ChipsPerAddOn": 0,
           "Levels":   [
           { "Banner": "SETTING UP", "Description": "PLAYERS: PLEASE TAKE YOUR SEATS", "DurationMinutes": 60, "IsBreak": true },
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
           "PrizePool": "1...$10,000,000\n2....$5,000,000\n..."
       }
    }
    $json$);


INSERT INTO structures (structure_id, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (1, $json$
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
