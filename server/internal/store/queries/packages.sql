-- The Question Bank is first-party and shared: no ownership scoping on
-- reads. preview = first question's text (A13 card); COALESCE(...)::text
-- keeps sqlc emitting plain string, never a nullable.

-- name: ListQuestionPackages :many
SELECT p.*,
       count(pq.id) AS question_count,
       COALESCE((SELECT pq2.text FROM package_questions pq2 WHERE pq2.package_id = p.id ORDER BY pq2.position, pq2.created_at LIMIT 1), '')::text AS preview
FROM question_packages p
LEFT JOIN package_questions pq ON pq.package_id = p.id
GROUP BY p.id
ORDER BY p.created_at, p.title;

-- Existence check so a missing package is distinguishable from an empty
-- import (PACKAGE_NOT_FOUND vs. zero copied rows).
-- name: GetQuestionPackage :one
SELECT * FROM question_packages WHERE id = $1;

-- The whole copy in one atomic statement; ownership in the WHERE (the
-- CreateQuestion pattern). row_number() — not raw pq.position — makes the
-- appended positions MAX+1..MAX+N regardless of gaps in package numbering;
-- the MAX subquery sees the pre-statement snapshot, so all N rows are
-- internally consistent.
-- name: ImportPackageQuestions :many
INSERT INTO questions (game_id, position, type, text, options, correct_option, accepted_answers, time_limit_seconds, imported_from_bank)
SELECT g.id,
       COALESCE((SELECT max(q.position) FROM questions q WHERE q.game_id = g.id), 0)
         + (row_number() OVER (ORDER BY pq.position, pq.created_at))::int,
       pq.type, pq.text, pq.options, pq.correct_option, pq.accepted_answers, pq.time_limit_seconds, true
FROM games g
JOIN package_questions pq ON pq.package_id = sqlc.arg(package_id)
WHERE g.id = sqlc.arg(game_id) AND g.organizer_id = sqlc.arg(organizer_id)
RETURNING *;
