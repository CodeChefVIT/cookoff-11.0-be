-- +goose Up
ALTER TABLE questions
ADD COLUMN bounty_active BOOLEAN NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE questions
DROP COLUMN bounty_active;
