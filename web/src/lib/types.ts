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
  createdAt: string
  updatedAt: string
}

export interface GameListItem {
  id: string
  title: string
  joinCode: string
  state: GameState
  questionCount: number
  createdAt: string
  updatedAt: string
}

export interface Game extends GameListItem {
  questions: Question[]
}

export interface GameList {
  items: GameListItem[]
}
