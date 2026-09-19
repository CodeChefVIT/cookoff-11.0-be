-- +goose Up
ALTER TABLE submission_results
ADD COLUMN stdout TEXT;

-- +goose Down
ALTER TABLE submission_results
DROP COLUMN stdout;