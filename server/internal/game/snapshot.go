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

// CurrentQuestion is the question a live control panel (or, from Epic 4, an
// Audience Display) renders while a round is in progress. It deliberately
// omits CorrectOption/AcceptedAnswers — always, at every state, including
// revealed — since Snapshot is the one payload both role=host and
// role=display receive over the same WS envelope; leaking the correct
// answer here would hand it out with no separate reveal gate to add later.
type CurrentQuestion struct {
	ID               string   `json:"id"`
	Position         int      `json:"position"`
	Type             string   `json:"type"`
	Text             string   `json:"text"`
	Options          []string `json:"options,omitempty"`
	TimeLimitSeconds int      `json:"timeLimitSeconds"`
	AnswerCutoffAt   string   `json:"answerCutoffAt"` // RFC 3339 UTC
	AnsweredCount    int      `json:"answeredCount"`
}
