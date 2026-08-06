package game

import (
	"sort"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

// ScoringConfig carries one game's points/bonus configuration
// (games.points_per_correct/speed_bonus_first/second/third, story 1.4)
// into AwardPoints.
type ScoringConfig struct {
	PointsPerCorrect int32
	SpeedBonusFirst  int32
	SpeedBonusSecond int32
	SpeedBonusThird  int32
}

// AwardPoints computes each answer's point award for one just-revealed
// Question (FR-17 epic AC-1). Every correct answer earns
// cfg.PointsPerCorrect; the first three correct answers ordered by
// ReceivedAt — ties broken by Seq, which is strictly increasing
// (answers.seq is a BIGINT GENERATED ALWAYS AS IDENTITY, so no two
// answers ever share one) — additionally earn SpeedBonusFirst/Second/Third.
// Incorrect answers earn 0. Fewer than three correct answers means fewer
// bonuses awarded, never a bonus reassigned to a lower place. Pure
// function, no I/O: every input is already resolved by the caller
// (game.Engine.Reveal) — mirrors grading.GradeMCQ/GradeExact's
// "pure function first" split.
func AwardPoints(answers []store.AnswerForScoring, cfg ScoringConfig) []store.AnswerPointsParams {
	correct := make([]store.AnswerForScoring, 0, len(answers))
	for _, a := range answers {
		if a.IsCorrect {
			correct = append(correct, a)
		}
	}
	sort.Slice(correct, func(i, j int) bool {
		if !correct[i].ReceivedAt.Equal(correct[j].ReceivedAt) {
			return correct[i].ReceivedAt.Before(correct[j].ReceivedAt)
		}
		return correct[i].Seq < correct[j].Seq
	})
	bonusByAnswerID := make(map[string]int32, 3)
	bonuses := [3]int32{cfg.SpeedBonusFirst, cfg.SpeedBonusSecond, cfg.SpeedBonusThird}
	for i, a := range correct {
		if i < len(bonuses) {
			bonusByAnswerID[a.AnswerID] = bonuses[i]
		}
	}
	points := make([]store.AnswerPointsParams, 0, len(answers))
	for _, a := range answers {
		p := int32(0)
		if a.IsCorrect {
			p = cfg.PointsPerCorrect + bonusByAnswerID[a.AnswerID]
		}
		points = append(points, store.AnswerPointsParams{AnswerID: a.AnswerID, Points: p})
	}
	return points
}

// LeaderboardEntry is one ranked row of the live leaderboard (FR-17/18,
// Snapshot's wire shape — see snapshot.go).
type LeaderboardEntry struct {
	ParticipantID string `json:"participantId"`
	DisplayName   string `json:"displayName"`
	Score         int32  `json:"score"`
	Rank          int    `json:"rank"`
}

// RankLeaderboard sorts scores descending and assigns standard
// competition ranks: equal Score shares one Rank, and the next distinct
// score's Rank skips ahead by the number of tied rows (1,1,3 — never
// 1,1,2) — epic AC-2's "equal scores share a rank". sort.SliceStable
// keeps tied participants in the order they arrived in scores; the
// caller (game.Engine.buildSnapshot) passes them in ListParticipants'
// joined_at-ASC order (store.GetLeaderboard mirrors that ORDER BY), so
// ties break by whoever joined first — deterministic, though not itself
// specified by the epic beyond "equal scores share a rank".
func RankLeaderboard(scores []store.ParticipantScore) []LeaderboardEntry {
	sorted := make([]store.ParticipantScore, len(scores))
	copy(sorted, scores)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })

	entries := make([]LeaderboardEntry, len(sorted))
	for i, s := range sorted {
		rank := i + 1
		if i > 0 && sorted[i-1].Score == s.Score {
			rank = entries[i-1].Rank
		}
		entries[i] = LeaderboardEntry{ParticipantID: s.ParticipantID, DisplayName: s.DisplayName, Score: s.Score, Rank: rank}
	}
	return entries
}
