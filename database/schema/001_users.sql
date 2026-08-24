-- +goose Up
CREATE TABLE users (
	id UUID NOT NULL,
	email TEXT NOT NULL UNIQUE, 
	reg_no TEXT NOT NULL UNIQUE,
	password TEXT NOT NULL,
	role TEXT NOT NULL,
	round_qualified INTEGER NOT NULL DEFAULT 0,
	google_id TEXT UNIQUE NULL,
    balance NUMERIC NOT NULL DEFAULT 0,
	score NUMERIC NOT NULL DEFAULT 0,
	name TEXT NOT NULL,
	is_banned BOOLEAN NOT NULL DEFAULT false,
	PRIMARY KEY(id)
);

-- +goose Down
DROP TABLE users;