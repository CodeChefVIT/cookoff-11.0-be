-- +goose Up
CREATE TABLE visual_solutions (
    id UUID NOT NULL,
    question_id UUID NOT NULL,
    solution  UUID[] NOT NULL,
    points NUMERIC NOT NULL,
    PRIMARY KEY(id)
);
-- +goose Down
DROP TABLE visual_solutions;

