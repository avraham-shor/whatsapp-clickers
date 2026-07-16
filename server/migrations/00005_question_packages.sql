-- +goose Up
-- FR-12 Question Bank: first-party, read-only in the pilot — content enters
-- via migrations only. package_questions mirrors questions' content-column
-- shape verbatim so the import is a column-for-column INSERT … SELECT.
CREATE TABLE question_packages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE package_questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Documented variance from the <singular>_id convention: the table
    -- already lives in the package_ namespace, question_package_id is noise.
    package_id UUID NOT NULL REFERENCES question_packages(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('mcq','free_text')),
    text TEXT NOT NULL,
    options TEXT[] NOT NULL DEFAULT '{}',
    correct_option INTEGER NOT NULL DEFAULT 0,
    accepted_answers TEXT[] NOT NULL DEFAULT '{}',
    time_limit_seconds INTEGER NOT NULL DEFAULT 20 CHECK (time_limit_seconds BETWEEN 5 AND 300),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT package_questions_type_shape CHECK (
        (type = 'mcq' AND cardinality(options) = 4 AND correct_option BETWEEN 1 AND 4 AND cardinality(accepted_answers) = 0)
        OR
        (type = 'free_text' AND cardinality(options) = 0 AND correct_option = 0 AND cardinality(accepted_answers) >= 1)
    )
);

CREATE INDEX idx_package_questions_package_id_position ON package_questions (package_id, position);

-- Provenance marker for the "מהמאגר" badge: one bit, survives edits (origin,
-- not sync state). Existing rows backfill false — they are custom questions.
ALTER TABLE questions ADD COLUMN imported_from_bank BOOLEAN NOT NULL DEFAULT false;

-- Seed pack (sample content — the real pilot pack remains OQ-4). Fixed UUID
-- so the question rows can reference it without RETURNING plumbing.
INSERT INTO question_packages (id, title)
VALUES ('11111111-1111-4111-8111-111111111111', 'טריוויה לכל המשפחה — חבילת פתיחה');

INSERT INTO package_questions (package_id, position, type, text, options, correct_option, accepted_answers, time_limit_seconds) VALUES
('11111111-1111-4111-8111-111111111111', 1, 'mcq', 'איזו חיה היא הגבוהה ביותר בעולם?', ARRAY['פיל','ג''ירפה','דוב','גמל'], 2, '{}', 20),
('11111111-1111-4111-8111-111111111111', 2, 'mcq', 'מי מהדמויות הבאות אינו אחד משלושת האבות?', ARRAY['אברהם','יצחק','יעקב','משה'], 4, '{}', 20),
('11111111-1111-4111-8111-111111111111', 3, 'free_text', 'על איזה הר ניתנה התורה?', '{}', 0, ARRAY['הר סיני','סיני'], 25),
('11111111-1111-4111-8111-111111111111', 4, 'mcq', 'כמה שנים הלכו בני ישראל במדבר?', ARRAY['עשרים','שלושים','ארבעים','חמישים'], 3, '{}', 25),
('11111111-1111-4111-8111-111111111111', 5, 'mcq', 'איזה חג נקרא גם חג האורים?', ARRAY['פסח','שבועות','סוכות','חנוכה'], 4, '{}', 20),
('11111111-1111-4111-8111-111111111111', 6, 'free_text', 'כמה נרות מדליקים בחנוכייה בכל שמונת ימי החנוכה יחד, כולל השמש?', '{}', 0, ARRAY['44','ארבעים וארבעה','ארבעים וארבע'], 45),
('11111111-1111-4111-8111-111111111111', 7, 'mcq', 'איזה מקום הוא הנמוך ביותר בעולם?', ARRAY['הכנרת','ים המלח','הים התיכון','מפרץ אילת'], 2, '{}', 25),
('11111111-1111-4111-8111-111111111111', 8, 'mcq', 'מי היה הכהן הגדול הראשון?', ARRAY['משה','אהרן','פינחס','שמואל'], 2, '{}', 30),
('11111111-1111-4111-8111-111111111111', 9, 'free_text', 'כמה פרקים יש בספר תהילים?', '{}', 0, ARRAY['150','מאה חמישים','ק"נ'], 40),
('11111111-1111-4111-8111-111111111111', 10, 'free_text', 'מה היו שמות שני בניו של יוסף?', '{}', 0, ARRAY['מנשה ואפרים','אפרים ומנשה'], 40);

-- +goose Down
ALTER TABLE questions DROP COLUMN imported_from_bank;
DROP TABLE package_questions;
DROP TABLE question_packages;
