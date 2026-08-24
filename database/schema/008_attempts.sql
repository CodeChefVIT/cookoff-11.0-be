-- +goose Up
CREATE TABLE attempts (
	id UUID NOT NULL,
	user_id UUID NOT NULL,
	question_id UUID NOT NULL,
	status TEXT NOT NULL DEFAULT 'available',
	is_buy_in_paid BOOLEAN NOT NULL DEFAULT false,
	attempted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	answered_at TIMESTAMPTZ,
	PRIMARY KEY(id),
    CONSTRAINT uq_attempts_user_question UNIQUE(user_id, question_id),
    CONSTRAINT chk_attempts_status CHECK (status IN ('available', 'bought', 'answered'))
);
 
-- +goose Down
DROP TABLE attempts;