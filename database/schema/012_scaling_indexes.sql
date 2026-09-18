-- +goose Up
CREATE INDEX idx_submissions_user_question ON submissions(user_id, question_id);
CREATE INDEX idx_attempts_question_id ON attempts(question_id);
CREATE INDEX idx_submissions_success ON submissions(user_id) WHERE LOWER(status) = 'success';
CREATE INDEX idx_submissions_submission_time ON submissions(submission_time);

-- +goose Down
DROP INDEX IF EXISTS idx_submissions_user_question;
DROP INDEX IF EXISTS idx_attempts_question_id;
DROP INDEX IF EXISTS idx_submissions_success;
DROP INDEX IF EXISTS idx_submissions_submission_time;