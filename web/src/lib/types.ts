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

/** Mirrors the Go game.CurrentQuestion. Deliberately omits the correct
 * answer (correctOption/acceptedAnswers) — Snapshot is the one payload both
 * role=host and role=display receive over the same WS envelope. */
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
}

/** The one WS wire message shape: server->client only, full snapshots. */
export interface SnapshotEnvelope {
  type: 'snapshot'
  seq: number
  state: LobbySnapshot
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
