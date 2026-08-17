-- +goose Up
CREATE TABLE questions (
	id UUID NOT NULL,
	description TEXT NOT NULL,
	title TEXT NOT NULL,
    qType TEXT NOT NULL,
	input_format TEXT[],
	buy_in NUMERIC, 
	reward NUMERIC,
	points INTEGER NOT NULL,
	round INTEGER NOT NULL,
	constraints TEXT[],
	output_format TEXT[],
    sample_test_input TEXT[],
    sample_test_output TEXT[],
    explanation TEXT[],
	PRIMARY KEY(id)
);

-- +goose Down
DROP TABLE questions;