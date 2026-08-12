// Wire types mirroring the Go payloads (camelCase). The server is the
// source of truth — fields irrelevant to a question's type are omitted.

// The seven canonical game states — identical strings in Go/TS/DB.
export type GameState =
  | 'draft'
  | 'lobby'
  | 'question_open'
  | 'question_closed'
  | 'revealed'
  | 'leaderboard'
  | 'finished'

export type QuestionType = 'mcq' | 'free_text'

export interface Question {
  id: string
  position: number
  type: QuestionType
  text: string
  /** Present on mcq only: exactly four options (letters are presentation). */
  options?: string[]
  /** Present on mcq only: 1-based index into options. */
  correctOption?: number
  /** Present on free_text only: the first entry is the primary form shown at Reveal. */
  acceptedAnswers?: string[]
  timeLimitSeconds: number
  /** Provenance marker: true when the question was copied from the Question Bank. */
  importedFromBank: boolean
  createdAt: string
  updatedAt: string
}

export interface GameListItem {
  id: string
  title: string
  joinCode: string
  state: GameState
  questionCount: number
  /** Scoring configuration (FR-17 config half); 0 disables a bonus. */
  pointsPerCorrect: number
  speedBonusFirst: number
  speedBonusSecond: number
  speedBonusThird: number
  createdAt: string
  updatedAt: string
}

export interface Game extends GameListItem {
  questions: Question[]
}

export interface GameList {
  items: GameListItem[]
}

/** Mirrors the Go game.QuestionReveal — what the Audience Display needs to
 * mark the answer, present ONLY while the game is `revealed` (story 4.4).
 * That state IS the gate: before the Organizer reveals, `reveal` is null and
 * the correct answer is nowhere on the wire.
 *
 * Not role-conditional: one snapshot serves both role=host and role=display
 * over the same WS envelope, and both are authenticated with the Organizer's
 * own session. Participants never receive a snapshot — they are on WhatsApp. */
export interface QuestionReveal {
  /** Present on mcq only: 1-based index into options. */
  correctOption?: number
  /** Present on free_text only: the primary form shown at Reveal (A15). */
  acceptedAnswer?: string
  /** Present on mcq only: answers per option, index-aligned with options. */
  optionCounts?: number[]
  /** Never omitted — 0 correct is a real number, and a dropped key would
   * render the free-text counts line as "undefined צדקו". */
  correctCount: number
}

/** Mirrors the Go game.CurrentQuestion. Deliberately omits the correct
 * answer (correctOption/acceptedAnswers) at every state EXCEPT revealed,
 * where the nested `reveal` below carries them — Snapshot is the one payload
 * both role=host and role=display receive over the same WS envelope, so the
 * state is the only gate there is, and it is the gate. */
export interface CurrentQuestion {
  id: string
  position: number
  type: QuestionType
  text: string
  options?: string[]
  timeLimitSeconds: number
  /** RFC 3339 UTC. */
  answerCutoffAt: string
  answeredCount: number
  /** Nullable AND optional, and the optionality is a deliberate, recorded
   * deviation from story 4.4's Task 4 (which asked for non-optional, to
   * mirror the Go pointer that carries no omitempty and therefore always
   * puts the key on the wire).
   *
   * Non-optional is a compile error in `question-stage.test.tsx` and
   * `display-page.test.tsx`, whose fixture builders predate this field — and
   * those two files are two of the four the same story forbids editing,
   * because a byte-identical diff on them is the whole proof that 4.4's two
   * component extractions preserved behaviour. The two requirements cannot
   * both hold literally.
   *
   * The optional form loses nothing the non-optional form was for: reading
   * `reveal.correctOption` without a check is still a type error, so derived
   * requirement 9's guard is still forced by the compiler. What it loses is
   * documentation fidelity to the wire — recovered by this comment, and by
   * the E2E, which asserts on the raw JSON's explicit `"reveal": null`. */
  reveal?: QuestionReveal | null
}

/** Mirrors the Go game.LeaderboardEntry — one ranked row of the live
 * leaderboard (FR-17/18). Equal scores share a rank. */
export interface LeaderboardEntry {
  participantId: string
  displayName: string
  score: number
  rank: number
}

/** Mirrors the Go game.DisplaySettings — room-level rendering settings
 * the Audience Display obeys. Set by the Organizer on the dashboard
 * because the audience cannot set prefers-reduced-motion on a projector. */
export interface DisplaySettings {
  reducedMotion: boolean
}

/** Mirrors the Go game.Snapshot — the REST open-lobby response body and the
 * WS envelope's "state" field share this one shape. */
export interface LobbySnapshot {
  gameId: string
  state: GameState
  joinCode: string
  platformNumber: string
  participantCount: number
  participants: { id: string; displayName: string }[]
  questionCount: number
  currentQuestion: CurrentQuestion | null
  leaderboard: LeaderboardEntry[]
  /** Non-optional: the server always sends it (no omitempty). */
  displaySettings: DisplaySettings
}

/** The one WS wire message shape: server->client only, full snapshots. */
export interface SnapshotEnvelope {
  type: 'snapshot'
  seq: number
  state: LobbySnapshot
}

/** Mirrors the Go questionStatsPayload — one Question's post-game
 * response counts (FR-14). answeredCount counts every recorded answer;
 * correctCount counts only those graded correct, so an answer still
 * ungraded is answered-but-not-correct. */
export interface QuestionStats {
  id: string
  position: number
  type: QuestionType
  text: string
  answeredCount: number
  correctCount: number
}

/** Mirrors the Go resultsPayload — the whole post-game summary in one
 * REST response. playerCount is the response-rate denominator: the
 * number of player-role Participants (Spectators excluded — they never
 * answer). */
export interface GameResults {
  gameId: string
  title: string
  state: GameState
  playerCount: number
  leaderboard: LeaderboardEntry[]
  questions: QuestionStats[]
}

/** A Question Bank package card: title, count, and a first-question preview (A13). */
export interface QuestionPackage {
  id: string
  title: string
  questionCount: number
  preview: string
}

export interface QuestionPackageList {
  items: QuestionPackage[]
}
