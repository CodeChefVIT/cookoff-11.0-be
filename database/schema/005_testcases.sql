-- +goose Up
CREATE TABLE testcases (
	id UUID NOT NULL UNIQUE,
	expected_output TEXT NOT NULL ,
	memory NUMERIC NOT NULL ,
	input TEXT NOT NULL ,
	hidden BOOLEAN NOT NULL ,
	runtime DECIMAL NOT NULL ,
	question_id UUID NOT NULL,
	PRIMARY KEY(id)
);

-- +goose Down
DROP TABLE testcases;