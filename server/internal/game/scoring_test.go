package game

import (
	"testing"
	"time"

	"github.com/avraham-shor/whatsapp-clickers/internal/store"
)

var scoringTestCfg = ScoringConfig{
	PointsPerCorrect: 100,
	SpeedBonusFirst:  50,
	SpeedBonusSecond: 30,
	SpeedBonusThird:  10,
}

func scoringTestTime(offsetSeconds int) time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(time.Duration(offsetSeconds) * time.Second)
}

func pointsByAnswerID(points []store.AnswerPointsParams) map[string]int32 {
	out := make(map[string]int32, len(points))
	for _, p := range points {
		out[p.AnswerID] = p.Points
	}
	return out
}

func TestAwardPointsGivesTopThreeCorrectSpeedBonusesInReceiptOrder(t *testing.T) {
	answers := []store.AnswerForScoring{
		{AnswerID: "a1", IsCorrect: true, ReceivedAt: scoringTestTime(1), Seq: 1},
		{AnswerID: "a2", IsCorrect: true, ReceivedAt: scoringTestTime(2), Seq: 2},
		{AnswerID: "a3", IsCorrect: true, ReceivedAt: scoringTestTime(3), Seq: 3},
		{AnswerID: "a4", IsCorrect: true, ReceivedAt: scoringTestTime(4), Seq: 4},
		{AnswerID: "a5", IsCorrect: true, ReceivedAt: scoringTestTime(5), Seq: 5},
		{AnswerID: "a6", IsCorrect: false, ReceivedAt: scoringTestTime(0), Seq: 6},
	}
	got := pointsByAnswerID(AwardPoints(answers, scoringTestCfg))

	want := map[string]int32{
		"a1": 100 + 50,
		"a2": 100 + 30,
		"a3": 100 + 10,
		"a4": 100,
		"a5": 100,
		"a6": 0,
	}
	// Length first: iterating want and reading got[id] cannot distinguish
	// "scored 0" from "absent", so dropping incorrect answers entirely
	// would satisfy the a6 expectation while leaving points_awarded NULL
	// for every wrong answer (migration 00014: NULL means not revealed).
	if len(got) != len(want) {
		t.Fatalf("got %d awards, want %d — every answer must be scored, including incorrect ones", len(got), len(want))
	}
	for id, wantPoints := range want {
		if got[id] != wantPoints {
			t.Errorf("points[%q] = %d, want %d", id, got[id], wantPoints)
		}
	}
}

// AC-1 names received_at as the PRIMARY ordering key and seq only as the
// tie-break. Every other test here happens to list answers whose Seq
// order matches their ReceivedAt order, which makes the two keys
// indistinguishable — a comparator that dropped ReceivedAt entirely and
// sorted by Seq alone passed the whole suite. Here Seq order is the exact
// inverse of receipt order, so only a received_at-primary sort can pass.
func TestAwardPointsOrdersByReceivedAtNotBySeq(t *testing.T) {
	answers := []store.AnswerForScoring{
		{AnswerID: "slowest-but-lowest-seq", IsCorrect: true, ReceivedAt: scoringTestTime(30), Seq: 1},
		{AnswerID: "middle", IsCorrect: true, ReceivedAt: scoringTestTime(20), Seq: 2},
		{AnswerID: "fastest-but-highest-seq", IsCorrect: true, ReceivedAt: scoringTestTime(10), Seq: 3},
	}
	got := pointsByAnswerID(AwardPoints(answers, scoringTestCfg))

	want := map[string]int32{
		"fastest-but-highest-seq": 100 + 50,
		"middle":                  100 + 30,
		"slowest-but-lowest-seq":  100 + 10,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d awards, want %d", len(got), len(want))
	}
	for id, wantPoints := range want {
		if got[id] != wantPoints {
			t.Errorf("points[%q] = %d, want %d (bonuses follow received_at, not seq)", id, got[id], wantPoints)
		}
	}
}

func TestAwardPointsBreaksReceiptTiesBySeq(t *testing.T) {
	tied := scoringTestTime(1)
	answers := []store.AnswerForScoring{
		{AnswerID: "later-seq", IsCorrect: true, ReceivedAt: tied, Seq: 20},
		{AnswerID: "earlier-seq", IsCorrect: true, ReceivedAt: tied, Seq: 10},
	}
	got := pointsByAnswerID(AwardPoints(answers, scoringTestCfg))

	if got["earlier-seq"] != 100+50 {
		t.Errorf("points[earlier-seq] = %d, want %d (lower Seq wins the earlier bonus slot)", got["earlier-seq"], 100+50)
	}
	if got["later-seq"] != 100+30 {
		t.Errorf("points[later-seq] = %d, want %d", got["later-seq"], 100+30)
	}
}

func TestAwardPointsWithFewerThanThreeCorrectAnswersOnlyBonusesThoseThatExist(t *testing.T) {
	answers := []store.AnswerForScoring{
		{AnswerID: "a1", IsCorrect: true, ReceivedAt: scoringTestTime(1), Seq: 1},
		{AnswerID: "a2", IsCorrect: false, ReceivedAt: scoringTestTime(2), Seq: 2},
		{AnswerID: "a3", IsCorrect: false, ReceivedAt: scoringTestTime(3), Seq: 3},
	}
	got := pointsByAnswerID(AwardPoints(answers, scoringTestCfg))

	if got["a1"] != 100+50 {
		t.Errorf("points[a1] = %d, want %d", got["a1"], 100+50)
	}
	if got["a2"] != 0 || got["a3"] != 0 {
		t.Errorf("incorrect answers got points[a2]=%d points[a3]=%d, want 0, 0", got["a2"], got["a3"])
	}
}

func TestAwardPointsZeroCorrectAnswersReturnsAllZero(t *testing.T) {
	answers := []store.AnswerForScoring{
		{AnswerID: "a1", IsCorrect: false, ReceivedAt: scoringTestTime(1), Seq: 1},
		{AnswerID: "a2", IsCorrect: false, ReceivedAt: scoringTestTime(2), Seq: 2},
	}
	got := AwardPoints(answers, scoringTestCfg)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for _, p := range got {
		if p.Points != 0 {
			t.Errorf("points[%q] = %d, want 0", p.AnswerID, p.Points)
		}
	}
}

// Input is deliberately NOT already in descending order. The rank loop
// only compares adjacent entries, so the descending sort is load-bearing
// for AC-2 — with pre-sorted input a no-op sort passed this test, and
// non-adjacent equal scores would not have shared a rank.
func TestRankLeaderboardSharesRankAcrossTiedScoresAndSkipsAhead(t *testing.T) {
	scores := []store.ParticipantScore{
		{ParticipantID: "p3", DisplayName: "Carol", Score: 50},
		{ParticipantID: "p1", DisplayName: "Alice", Score: 100},
		{ParticipantID: "p2", DisplayName: "Bob", Score: 100},
	}
	entries := RankLeaderboard(scores)

	want := []LeaderboardEntry{
		{ParticipantID: "p1", DisplayName: "Alice", Score: 100, Rank: 1},
		{ParticipantID: "p2", DisplayName: "Bob", Score: 100, Rank: 1},
		{ParticipantID: "p3", DisplayName: "Carol", Score: 50, Rank: 3},
	}
	// Length first: ranging over entries rather than want meant a
	// regression that returned zero or one row never reached the tie case
	// and reported PASS.
	if len(entries) != len(want) {
		t.Fatalf("RankLeaderboard returned %d entries (%+v), want %d", len(entries), entries, len(want))
	}
	for i := range want {
		if entries[i] != want[i] {
			t.Errorf("entries[%d] = %+v, want %+v", i, entries[i], want[i])
		}
	}
}

func TestRankLeaderboardBreaksTiesByInputOrder(t *testing.T) {
	scores := []store.ParticipantScore{
		{ParticipantID: "joined-first", DisplayName: "Alice", Score: 100},
		{ParticipantID: "joined-second", DisplayName: "Bob", Score: 100},
	}
	entries := RankLeaderboard(scores)
	if entries[0].ParticipantID != "joined-first" || entries[1].ParticipantID != "joined-second" {
		t.Errorf("tie order = [%q, %q], want input order preserved [joined-first, joined-second]", entries[0].ParticipantID, entries[1].ParticipantID)
	}
}
