-- +goose Up
CREATE TABLE visual_solutions (
    id UUID NOT NULL UNIQUE,
    question_id UUID NOT NULL,
    solution  UUID[] NOT NULL,
    points NUMERIC NOT NULL,
);
-- +goose Down
DROP TABLE visual_solutions;

