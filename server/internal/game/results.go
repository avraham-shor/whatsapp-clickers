package game

import (
	"context"
	"fmt"

	"github.com/avraham-shor/whatsapp-clickers/internal/store/gen"
)

// PersonalResult is one answering Participant's resolved inputs to the
// Result message templates (wa/messages_he.go's four Result rows) — the
// WhatsApp personal-result dispatch's data source (FR-6, story 3.8).
// BasePoints is the Game's configured PointsPerCorrect when IsCorrect (0
// otherwise); BonusPoints is the Speed Bonus component only (0 when no
// bonus applied). The templates show these as two separate numbers,
// never their sum (EXPERIENCE.md's "Result — correct + bonus" row) —
// that is why this struct splits them instead of carrying one Points
// total.
type PersonalResult struct {
	Phone       string
	IsCorrect   bool
	BasePoints  int32
	BonusPoints int32
	Rank        int
}

// RevealedQuestionResults is ResultsForRevealedQuestion's return value.
type RevealedQuestionResults struct {
	// Results holds one entry per Participant who answered the revealed
	// question. Non-answerers are absent by construction —
	// ListAnswerResultsForQuestion only returns rows with a recorded
	// answer — which IS epic AC-3's "no per-question message" silence
	// rule; the caller never filters anything out.
	Results []PersonalResult
	// CorrectAnswer is the human-readable correct answer shown in a
	// wrong-answer message: the correct MCQ option's text, or the
	// Free-Text question's primary (first) Accepted Answer
	// (EXPERIENCE.md A15 — the same "primary form" already used at
	// Reveal on the Audience Display stage, Epic 4).
	CorrectAnswer string
	// IsLastQuestion is true when the revealed question was the game's
	// final one — the wrong-answer message drops "עוד הכול פתוח" in
	// that case (EXPERIENCE.md A5).
	IsLastQuestion bool
}

// ResultsForRevealedQuestion resolves the WhatsApp personal-result
// dispatch's full data source (FR-6) for gameID's question at position.
//
// position is the caller's own already-committed Reveal snapshot's
// CurrentQuestion.Position — NOT re-derived from a fresh
// GetGameForOrganizer read of g.CurrentQuestionPosition. This matters:
// this method runs from httpapi/control.go's own goroutine, spawned
// AFTER the REST response is written, so an organizer could in
// principle click "next question" before this goroutine runs, which
// would advance g.CurrentQuestionPosition to a DIFFERENT question and —
// if this method re-read it fresh — silently compute results for the
// wrong question. Binding on the specific position the caller already
// knows was just revealed closes that race for the per-answer data, the
// same discipline RecordAnswer's own SQL comment describes ("binding on
// the specific question_id... closes a narrower race" —
// queries/answers.sql).
//
// It does NOT make this method a point-in-time snapshot, and an earlier
// version of this comment wrongly claimed it closed the race "entirely"
// (code review, story 3.8). GetLeaderboard below is deliberately
// unbound — it reads current cumulative standings, and the four store
// calls here share no transaction — so if a concurrent NextQuestion and
// a subsequent reveal land while this method runs, the ranks reported
// can already include a later question's points. That is acceptable and
// arguably correct: AC-1 asks for the Participant's "current rank", not
// their rank as of the revealed question. What must never drift is the
// per-answer grade/points data, and that is what position binds.
//
// g.PointsPerCorrect is safe to re-read fresh: it is a game-level
// config, not per-question, and handleUpdateScoring is
// requireDraftGame-guarded, so it cannot change once a game has started.
//
// Called only after Reveal has committed — from httpapi/control.go's
// own goroutine, mirroring dispatchQuestionOpened's placement (story
// 3.2) exactly. organizerID scopes the question lookup; like
// PlayerRecipients, this trusts the caller's own guard
// (snapshot.State == StateRevealed) rather than re-checking state here.
func (e *Engine) ResultsForRevealedQuestion(ctx context.Context, gameID, organizerID string, position int32) (RevealedQuestionResults, error) {
	g, err := e.store.GetGameForOrganizer(ctx, gameID, organizerID)
	if err != nil {
		return RevealedQuestionResults{}, err
	}
	questions, err := e.store.ListQuestionsByGame(ctx, gameID, organizerID)
	if err != nil {
		return RevealedQuestionResults{}, err
	}

	var current gen.Question
	found := false
	for _, q := range questions {
		if q.Position == position {
			current = q
			found = true
			break
		}
	}
	if !found {
		// Positions are a dense 1..N sequence (story 3.1 / DeleteQuestion's
		// gap-closing) and the caller only ever passes a position it just
		// saw revealed — unreachable in practice; fail loudly rather than
		// dispatch with a blank correct answer.
		return RevealedQuestionResults{}, fmt.Errorf("game: no question at position %d for game %s", position, gameID)
	}

	var correctAnswer string
	switch current.Type {
	case "mcq":
		if current.CorrectOption < 1 || int(current.CorrectOption) > len(current.Options) {
			return RevealedQuestionResults{}, fmt.Errorf("game: question %s has out-of-range correct_option %d", current.ID, current.CorrectOption)
		}
		correctAnswer = current.Options[current.CorrectOption-1]
	case "free_text":
		if len(current.AcceptedAnswers) == 0 {
			return RevealedQuestionResults{}, fmt.Errorf("game: free_text question %s has no accepted answers", current.ID)
		}
		correctAnswer = current.AcceptedAnswers[0]
	default:
		// questions_type_shape's CHECK constraint guarantees this never
		// happens for a real row (same invariant RecordAnswer already
		// trusts) — fail closed with an error rather than guess.
		return RevealedQuestionResults{}, fmt.Errorf("game: question %s has unrecognized type %q", current.ID, current.Type)
	}
	// "Last question" is the absence of a question at position+1 — the
	// exact test NextQuestion uses to decide between OpenNextQuestion and
	// FinishGame. An earlier version compared position to len(questions),
	// which silently disagrees with NextQuestion whenever positions are
	// not a dense 1..N sequence: the room would be told "עוד הכול פתוח"
	// on the real final question, moments before the game ended. Mirror
	// the state machine rather than assuming density (code review, story
	// 3.8).
	isLastQuestion := true
	for _, q := range questions {
		if q.Position == position+1 {
			isLastQuestion = false
			break
		}
	}

	scores, err := e.store.GetLeaderboard(ctx, gameID)
	if err != nil {
		return RevealedQuestionResults{}, err
	}
	rankByParticipant := make(map[string]int, len(scores))
	for _, entry := range RankLeaderboard(scores) {
		rankByParticipant[entry.ParticipantID] = entry.Rank
	}

	rows, err := e.store.ListAnswerResultsForQuestion(ctx, gameID, position)
	if err != nil {
		return RevealedQuestionResults{}, err
	}
	results := make([]PersonalResult, 0, len(rows))
	for _, r := range rows {
		var base, bonus int32
		if r.IsCorrect {
			base = g.PointsPerCorrect
			if r.Points > base {
				bonus = r.Points - base
			}
		}
		// A map miss would yield rank 0 and put "מקום 0 בטבלה" — a rank
		// that cannot exist — on a real participant's phone. Every other
		// anomaly in this function fails loudly rather than dispatching
		// something wrong, and this is the one that reaches a human, so it
		// does too. Unreachable as the roster stands (GetLeaderboard LEFT
		// JOINs every role='player' participant, and GetOpenQuestionForPlayer
		// restricts answering to that same role, so no answerer can lack an
		// entry) — this guards the invariant, it does not paper over a known
		// gap. Code review, story 3.8.
		rank, ok := rankByParticipant[r.ParticipantID]
		if !ok {
			return RevealedQuestionResults{}, fmt.Errorf("game: participant %s answered question at position %d of game %s but is absent from the leaderboard", r.ParticipantID, position, gameID)
		}
		results = append(results, PersonalResult{
			Phone:       r.Phone,
			IsCorrect:   r.IsCorrect,
			BasePoints:  base,
			BonusPoints: bonus,
			Rank:        rank,
		})
	}

	return RevealedQuestionResults{Results: results, CorrectAnswer: correctAnswer, IsLastQuestion: isLastQuestion}, nil
}
