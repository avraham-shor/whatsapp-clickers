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
}

// ParticipantSummary is the participant shape a lobby/live client renders;
// no role field yet — nothing consumes it until Stories 2.4/2.5.
type ParticipantSummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}
