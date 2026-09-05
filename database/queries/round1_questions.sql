--this query is already there in vihaan's pr just making it here to use it for the submit sequence

-- name: GetRoundOneVisualQuestion :one
SELECT id
FROM questions
WHERE id = $1
  AND round = 1
  AND LOWER(q_type) = 'visual';

-- name: ListVisualBlocksByQuestionID :many
SELECT id, question_id, content
FROM visual_blocks
WHERE question_id = $1
ORDER BY id ASC;
