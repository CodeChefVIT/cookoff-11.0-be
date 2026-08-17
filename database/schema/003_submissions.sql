-- +goose Up
CREATE TABLE submissions (
	id UUID NOT NULL UNIQUE,
	question_id UUID NOT NULL,
	testcases_passed INTEGER DEFAULT 0,
	testcases_failed INTEGER DEFAULT 0,
	runtime DECIMAL,
	submission_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	source_code TEXT NOT NULL,
	language_id INTEGER NOT NULL,
	description TEXT,
	memory NUMERIC,
	user_id UUID,
	status TEXT,
	PRIMARY KEY(id)
);

-- +goose Down
DROP TABLE submissions;