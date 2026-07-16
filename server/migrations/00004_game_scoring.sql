-- +goose Up
-- FR-17 configuration half: scoring lives on the game row for the engine
-- (story 3.7) to consume. NOT NULL DEFAULT backfills existing games with the
-- canonical defaults, so AC-1's pre-fill is schema truth, not a client
-- constant. Bounds mirror the handler checks (0 allowed — zero disables a
-- bonus; 10000 cap keeps worst-case cumulative scores far inside int32).
ALTER TABLE games ADD COLUMN points_per_correct INTEGER NOT NULL DEFAULT 100 CHECK (points_per_correct BETWEEN 0 AND 10000);
ALTER TABLE games ADD COLUMN speed_bonus_first INTEGER NOT NULL DEFAULT 50 CHECK (speed_bonus_first BETWEEN 0 AND 10000);
ALTER TABLE games ADD COLUMN speed_bonus_second INTEGER NOT NULL DEFAULT 30 CHECK (speed_bonus_second BETWEEN 0 AND 10000);
ALTER TABLE games ADD COLUMN speed_bonus_third INTEGER NOT NULL DEFAULT 20 CHECK (speed_bonus_third BETWEEN 0 AND 10000);

-- +goose Down
ALTER TABLE games DROP COLUMN points_per_correct;
ALTER TABLE games DROP COLUMN speed_bonus_first;
ALTER TABLE games DROP COLUMN speed_bonus_second;
ALTER TABLE games DROP COLUMN speed_bonus_third;
