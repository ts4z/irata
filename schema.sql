
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
       "StructureID": 1,
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
       "EventName": "Event Two",
       "Description": "Description for Event Two",
       "FooterPlugsID": 1,
       "StructureID": 1,
       "State": {
           "IsClockRunning": false,
           "CurrentLevelNumber": 0,
           "TimeRemainingMillis": 3599000,
           "CurrentPlayers": 33,
           "BuyIns": 33,
           "TotalChips": 9900,
           "PrizePool": "Event\nTwo\nFTW"
       }
    }
    $json$);

INSERT INTO structures (structure_id, model_data) 
OVERRIDING SYSTEM VALUE
VALUES (1, $json$
    {
       "Levels": [
           {
              "Description": "AWAITING START...",
              "DurationMinutes": 60,
              "IsBreak": true
           },
           {
              "Description": "25-50 + 50 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           },
           {
              "Description": "50-75 + 75 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           },
           {
              "Description": "50-100 + 100 ANTE",
              "Durationminutes": 18,
              "IsBreak": false
           },
           {
              "Description": "75-150 + 150 ANTE",
              "Durationminutes": 18,
              "IsBreak": false
           },
           {
              "Description": "100-200 + 200 ANTE",
              "Durationminutes": 18,
              "IsBreak": false
           },
           {
              "Description": "PICTURE TIME",
              "DurationMinutes": 20,
              "IsBreak": true
           },
           {
              "Description": "200-300 + 300 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           },
           {
              "Description": "200-400 + 400 ANTE",
              "DurationMinutes": 18,
              "IsBreak": false
           }
       ]
    }
    $json$ );
