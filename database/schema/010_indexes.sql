-- +goose Up
CREATE INDEX idx_submissions_user_id ON submissions(user_id);
CREATE INDEX idx_submissions_question_id ON submissions(question_id);
CREATE INDEX idx_submission_results_submission_id ON submission_results(submission_id);
CREATE INDEX idx_submission_results_testcase_id ON submission_results(testcase_id);
CREATE INDEX idx_testcases_question_id ON testcases(question_id);
CREATE INDEX idx_questions_round ON questions(round);
CREATE INDEX idx_visual_solutions_question_id ON visual_solutions(question_id);
CREATE INDEX idx_visual_blocks_question_id ON visual_blocks(question_id);

-- +goose Down
DROP INDEX IF EXISTS idx_visual_blocks_question_id;
DROP INDEX IF EXISTS idx_visual_solutions_question_id;
DROP INDEX IF EXISTS idx_questions_round;
DROP INDEX IF EXISTS idx_testcases_question_id;
DROP INDEX IF EXISTS idx_submission_results_testcase_id;
DROP INDEX IF EXISTS idx_submission_results_submission_id;
DROP INDEX IF EXISTS idx_submissions_question_id;
DROP INDEX IF EXISTS idx_submissions_user_id;
