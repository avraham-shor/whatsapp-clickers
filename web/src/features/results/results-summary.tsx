import { Link } from 'react-router'
import { useQuery } from '@tanstack/react-query'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { GameResults } from '@/lib/types'

interface ResultsSummaryProps {
  gameId: string
}

// A game that ended with nobody on the roster is ordinary (open the lobby,
// nobody joins, stop) — the rate has no denominator, so show a dash rather
// than a division by zero.
function responsePercent(answered: number, players: number): string {
  if (players === 0) return '—'
  return `${Math.round((answered / players) * 100)}%`
}

/**
 * The Organizer's post-game summary (FR-14): final Leaderboard plus
 * per-question response rates. Owns its own REST fetch so both mount
 * points — the routed /games/:gameId/results page and ControlPage's
 * finished branch — render identically.
 *
 * Deliberately REST and not the WS snapshot: the summary must survive a
 * reload, a different browser, and a fresh sign-in (epic AC-2), none of
 * which have a snapshot, and `finished` is terminal so nothing
 * re-broadcasts. Per-question rates are not in the snapshot at all.
 *
 * Renders no top-level heading — each mount point owns its own <h1>
 * (results.title when routed, live.gameOverTitle on the control panel);
 * the two <caption>s are this component's section headings.
 */
export function ResultsSummary({ gameId }: ResultsSummaryProps) {
  const results = useQuery({
    // A child of the ['games', gameId] key, so invalidating the game also
    // invalidates its results.
    queryKey: ['games', gameId, 'results'],
    queryFn: () => api<GameResults>(`/api/games/${gameId}/results`),
  })

  if (results.isPending) {
    return <p className="text-host-text-secondary">{strings.common.loading}</p>
  }

  if (results.isError) {
    if (results.error instanceof ApiError && results.error.status === 404) {
      return (
        <div className="flex flex-col items-start gap-2">
          <p className="text-host-text">{strings.gameEditor.notFound}</p>
          <Link to="/" className="text-green-800 underline">
            {strings.gameEditor.backToGames}
          </Link>
        </div>
      )
    }
    // Checked by code, not by the 409 status — control-page.tsx's
    // GRADING_INCOMPLETE handling sets the precedent, and for the same
    // reason: other 409s must not borrow this message.
    if (results.error instanceof ApiError && results.error.code === 'GAME_NOT_FINISHED') {
      return (
        <p role="alert" className="text-host-text">
          {strings.results.notFinished}
        </p>
      )
    }
    return (
      <p role="alert" className="text-sm text-error">
        {strings.results.loadError}
      </p>
    )
  }

  const { title, playerCount, leaderboard, questions } = results.data

  return (
    <div className="flex flex-col gap-6">
      {/* Names the game these numbers belong to. Load-bearing on the routed
          mount, where the organizer arrives cold from a bookmark or a fresh
          sign-in (epic AC-2) with no context but the URL — the <h1> above is
          a static label, so without this the page is durable but anonymous.
          Each mount point owns its own <h1>; this is that heading's
          subtitle, and the section headings below sit under it. */}
      <h2 className="text-lg font-heading text-host-text">{title}</h2>

      {/* Suppressed at zero: noPlayers below already says nobody joined, so
          a zero-count label alongside it states the same fact twice. */}
      {playerCount > 0 && (
        <p className="text-host-text-secondary">
          {/* Plain <bdi>, not dir="ltr" — a Hebrew phrase with an embedded
              digit, the same treatment responseRate gets below. */}
          <bdi>{strings.results.playerCountLabel(playerCount)}</bdi>
        </p>
      )}

      <section className="flex flex-col gap-3 rounded-md border border-host-border bg-surface-raised p-4">
        {leaderboard.length === 0 ? (
          <>
            <h3 className="font-heading text-host-text">{strings.results.leaderboardTitle}</h3>
            <p className="text-host-text-secondary">{strings.results.noPlayers}</p>
          </>
        ) : (
          <table className="w-full text-start">
            {/* The heading sits inside the caption rather than beside it: a
                <caption> is not exposed as a heading, so without this the
                populated page had nothing between the page <h1> and the
                rows while the empty page had one — the outline collapsed
                exactly where there was something to navigate. */}
            <caption className="pb-3 text-start">
              <h3 className="font-heading text-host-text">{strings.results.leaderboardTitle}</h3>
            </caption>
            <thead>
              <tr className="border-b border-host-border text-sm text-host-text-secondary">
                <th scope="col" className="p-3 text-start font-ui">
                  {strings.results.rankColumn}
                </th>
                <th scope="col" className="p-3 text-start font-ui">
                  {strings.results.participantColumn}
                </th>
                <th scope="col" className="p-3 text-start font-ui">
                  {strings.results.scoreColumn}
                </th>
              </tr>
            </thead>
            <tbody>
              {/* Already rank-sorted server-side (game.RankLeaderboard) —
                  never re-sorted here. */}
              {leaderboard.map((entry) => (
                <tr key={entry.participantId} className="border-b border-host-border last:border-b-0">
                  <td className="p-3 text-host-text">
                    {/* Digits inside RTL Hebrew reorder visibly without
                        isolation (DESIGN.md bidi) — every number on this
                        page is wrapped. */}
                    <bdi dir="ltr">{entry.rank}</bdi>
                  </td>
                  <td className="p-3 text-host-text">{entry.displayName}</td>
                  <td className="p-3 text-host-text">
                    <bdi dir="ltr">{entry.score}</bdi>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="flex flex-col gap-3 rounded-md border border-host-border bg-surface-raised p-4">
        {questions.length === 0 ? (
          <>
            <h3 className="font-heading text-host-text">{strings.results.questionsTitle}</h3>
            <p className="text-host-text-secondary">{strings.results.noQuestions}</p>
          </>
        ) : (
          <table className="w-full text-start">
            <caption className="pb-3 text-start">
              <h3 className="font-heading text-host-text">{strings.results.questionsTitle}</h3>
            </caption>
            <thead>
              <tr className="border-b border-host-border text-sm text-host-text-secondary">
                <th scope="col" className="p-3 text-start font-ui">
                  {strings.results.questionColumn}
                </th>
                <th scope="col" className="p-3 text-start font-ui">
                  {strings.results.answeredColumn}
                </th>
                <th scope="col" className="p-3 text-start font-ui">
                  {strings.results.correctColumn}
                </th>
              </tr>
            </thead>
            <tbody>
              {questions.map((question) => (
                <tr key={question.id} className="border-b border-host-border last:border-b-0">
                  <td className="p-3 text-host-text">
                    <span className="text-host-text-secondary">
                      <bdi dir="ltr">{question.position}</bdi>
                    </span>{' '}
                    {question.text}
                  </td>
                  <td className="p-3 text-host-text">
                    {/* Plain <bdi>, NOT dir="ltr": this is a Hebrew phrase
                        with embedded digits, so it isolates from the
                        adjacent percentage while keeping its own RTL
                        direction — forcing LTR here would reposition the
                        phrase's Hebrew word. Only standalone numeric
                        tokens get dir="ltr". */}
                    <bdi>{strings.results.responseRate(question.answeredCount, playerCount)}</bdi>{' '}
                    <span className="text-sm text-host-text-secondary">
                      <bdi dir="ltr">{responsePercent(question.answeredCount, playerCount)}</bdi>
                    </span>
                  </td>
                  <td className="p-3 text-host-text">
                    <bdi dir="ltr">{question.correctCount}</bdi>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  )
}
