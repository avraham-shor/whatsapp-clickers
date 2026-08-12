package game

// Snapshot is the full live-state payload every lobby/live surface renders
// from — the WS wire's "state" field and the REST open-lobby response body
// share this one shape.
type Snapshot struct {
	GameID           string               `json:"gameId"`
	State            string               `json:"state"`
	JoinCode         string               `json:"joinCode"`
	PlatformNumber   string               `json:"platformNumber"`
	ParticipantCount int                  `json:"participantCount"`
	Participants     []ParticipantSummary `json:"participants"`
	QuestionCount    int                  `json:"questionCount"`
	CurrentQuestion  *CurrentQuestion     `json:"currentQuestion"`
	Leaderboard      []LeaderboardEntry   `json:"leaderboard"`
	DisplaySettings  DisplaySettings      `json:"displaySettings"`
}

// DisplaySettings carries the room-level rendering settings the Audience
// Display obeys (FR-9, story 4.1). It rides the snapshot rather than a
// display-side control because the room shares one projector and nobody
// interacts with it — the Organizer decides for everyone.
type DisplaySettings struct {
	ReducedMotion bool `json:"reducedMotion"`
}

// ParticipantSummary is the participant shape a lobby/live client renders;
// no role field yet — nothing consumes it until Stories 2.4/2.5.
type ParticipantSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// QuestionReveal carries what the Audience Display needs to mark the
// answer, and it is present ONLY while games.state = 'revealed' (story
// 4.4). That state IS the gate: before the Organizer reveals, the field
// is nil and the correct answer is nowhere on the wire, which is the
// property CurrentQuestion's doc comment used to hold absolutely.
//
// Not role-conditional: one Snapshot serves both role=host and
// role=display over the same WS envelope, and both are authenticated
// with the Organizer's own session. Participants never receive a
// snapshot at all — they are on WhatsApp.
type QuestionReveal struct {
	// 1-based index into Options; mcq only. omitempty drops the
	// free_text sentinel 0 (questions_type_shape guarantees 1..4 for
	// mcq, so a real value is never dropped).
	CorrectOption int `json:"correctOption,omitempty"`
	// The first Accepted Answer — "the primary form shown at Reveal"
	// (EXPERIENCE.md Content Rules, A15); free_text only.
	AcceptedAnswer string `json:"acceptedAnswer,omitempty"`
	// Answers per option, index-aligned with Options; mcq only.
	OptionCounts []int `json:"optionCounts,omitempty"`
	// Answers graded correct. NO omitempty: 0 correct is a real and
	// interesting number, and dropping it would make the free-text
	// counts line render its count as "undefined".
	CorrectCount int `json:"correctCount"`
}

// CurrentQuestion is the question a live control panel (or, from Epic 4, an
// Audience Display) renders while a round is in progress. It deliberately
// omits CorrectOption/AcceptedAnswers at every state EXCEPT revealed, where
// the nested Reveal below carries them — Snapshot is the one payload both
// role=host and role=display receive over the same WS envelope, so the
// state is the only gate there is, and it is the gate (story 4.4).
type CurrentQuestion struct {
	ID               string   `json:"id"`
	Position         int      `json:"position"`
	Type             string   `json:"type"`
	Text             string   `json:"text"`
	Options          []string `json:"options,omitempty"`
	TimeLimitSeconds int      `json:"timeLimitSeconds"`
	AnswerCutoffAt   string   `json:"answerCutoffAt"` // RFC 3339 UTC
	AnsweredCount    int      `json:"answeredCount"`
	// Pointer with no omitempty, so the wire always carries
	// "reveal": null before the reveal rather than omitting the key —
	// an absent key and a null both read as null in TS, but an explicit
	// null is self-documenting in a captured frame.
	Reveal *QuestionReveal `json:"reveal"`
}
