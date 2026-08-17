-- +goose Up
CREATE TABLE visual_blocks (
    id UUID NOT NULL UNIQUE,
    question_id UUID NOT NULL,
    content TEXT NOT NULL,
    PRIMARY KEY(id)
    );
-- +goose Down
DROP TABLE visual_blocks;
