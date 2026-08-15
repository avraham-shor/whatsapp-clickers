import { strings } from '@/lib/strings.he'
import type { StageProps } from './stage-props'

/** [A16] and DESIGN.md's winner-card.name both cap the stacked names at
 *  three. A fourth simultaneous winner is reachable (a small pilot game's
 *  last question can tie four ways at rank 1) and neither spec resolves it
 *  beyond "up to 3" — so the fourth is CUT, with no "+N more" affordance.
 *  The same accepted-cost posture as 4.5's top-ten boundary tie, and for
 *  the same reason: inventing overflow UI for a case no spec addresses is
 *  a bigger risk than the cut. */
const maxNames = 3

type PieceColor = 'gold' | 'green'
/** The mockup's two class slots (cf-circle + cf-sm/md/lg, or the
 *  single-measure cf-bar / cf-sq) collapsed into ONE key. Deliberate: only
 *  circles have a size variant, so a separate optional `size` field would
 *  need a runtime fallback for bars and squares that the type system could
 *  never discharge. One exhaustive Record instead, with no default arm. */
type PieceShape = 'circle-sm' | 'circle-md' | 'circle-lg' | 'bar' | 'square'

interface ConfettiPiece {
  left: string
  top: string
  duration: string
  delay: string
  color: PieceColor
  shape: PieceShape
}

/** Whole literals, never composed from a template: Tailwind v4 scans SOURCE
 *  TEXT, so `bg-${color}` generates nothing at all (4.5's Dev Notes record
 *  the same trap).
 *
 *  Every measure is the mockup's cqw value with the unit renamed to vw and
 *  the magnitude untouched — 1cqw = 1vw at this project's 1920 design width
 *  and no container-type is declared anywhere in this codebase (derived
 *  requirement 11). */
const shapeClass: Record<PieceShape, string> = {
  'circle-sm': 'h-[0.7vw] w-[0.7vw] rounded-full',
  'circle-md': 'h-[0.9vw] w-[0.9vw] rounded-full',
  'circle-lg': 'h-[1.05vw] w-[1.05vw] rounded-full',
  bar: 'h-[1.25vw] w-[0.55vw] rounded-[0.15vw]',
  square: 'h-[0.8vw] w-[0.8vw] rounded-[0.15vw]',
}

/** Gold and green-600 only (UX-DR4 / DESIGN.md winner-card.celebration).
 *  This and the winner name below are the only gold on the whole product
 *  besides the timer's ≤5s ring — UX-DR2 rations it to two moments. */
const colorClass: Record<PieceColor, string> = {
  gold: 'bg-gold',
  green: 'bg-green-600',
}

/** mockups/key-stage-winner.html's 22 pieces, transcribed verbatim rather
 *  than generated: the scatter is what was actually reviewed, and a
 *  programmatic layout would be a different picture that merely resembles
 *  it. `left`/`top` are already percentages in the mockup and need no
 *  conversion at all.
 *
 *  Every delay is NEGATIVE, which is the mockup's own device and not a
 *  typo: it starts each piece part-way through its drift, so the very first
 *  painted frame already reads as a scattered celebration instead of 22
 *  pieces queued above the screen. That is also what makes the
 *  reduced-motion frame — the same coordinates with no animation at all —
 *  a believable still rather than an empty one. */
const confettiPieces: ConfettiPiece[] = [
  { left: '4%', top: '18%', duration: '11s', delay: '-5s', color: 'gold', shape: 'circle-md' },
  { left: '9%', top: '62%', duration: '13s', delay: '-7s', color: 'green', shape: 'bar' },
  { left: '14%', top: '35%', duration: '10s', delay: '-4.5s', color: 'gold', shape: 'bar' },
  { left: '19%', top: '78%', duration: '12s', delay: '-6.5s', color: 'green', shape: 'circle-sm' },
  { left: '24%', top: '12%', duration: '14s', delay: '-6s', color: 'gold', shape: 'square' },
  { left: '29%', top: '55%', duration: '11s', delay: '-5.8s', color: 'green', shape: 'bar' },
  { left: '34%', top: '30%', duration: '12.5s', delay: '-5.5s', color: 'gold', shape: 'circle-lg' },
  { left: '39%', top: '70%', duration: '10.5s', delay: '-5s', color: 'green', shape: 'square' },
  { left: '44%', top: '20%', duration: '13.5s', delay: '-7.2s', color: 'gold', shape: 'bar' },
  { left: '49%', top: '84%', duration: '11.5s', delay: '-5.2s', color: 'gold', shape: 'circle-md' },
  { left: '54%', top: '45%', duration: '12s', delay: '-6.8s', color: 'green', shape: 'circle-md' },
  { left: '59%', top: '15%', duration: '10s', delay: '-4.2s', color: 'gold', shape: 'bar' },
  { left: '64%', top: '66%', duration: '13s', delay: '-6s', color: 'green', shape: 'bar' },
  { left: '69%', top: '38%', duration: '11s', delay: '-6.2s', color: 'gold', shape: 'circle-md' },
  { left: '74%', top: '80%', duration: '12.5s', delay: '-5.9s', color: 'gold', shape: 'square' },
  { left: '79%', top: '25%', duration: '10.5s', delay: '-4.8s', color: 'green', shape: 'circle-sm' },
  { left: '84%', top: '58%', duration: '13.5s', delay: '-6.4s', color: 'gold', shape: 'bar' },
  { left: '89%', top: '42%', duration: '11.5s', delay: '-6.1s', color: 'green', shape: 'bar' },
  { left: '94%', top: '72%', duration: '12s', delay: '-5.4s', color: 'gold', shape: 'circle-md' },
  { left: '7%', top: '88%', duration: '14s', delay: '-7.5s', color: 'gold', shape: 'bar' },
  { left: '51%', top: '8%', duration: '11s', delay: '-5.6s', color: 'green', shape: 'circle-sm' },
  { left: '91%', top: '10%', duration: '13s', delay: '-6.6s', color: 'gold', shape: 'circle-sm' },
]

// Shared by all three branches: the takeover IS the surface at `finished`,
// whether or not it has a winner to name. absolute inset-0 resolves against
// the shell's padding box, so the projector safe margin is restored inside.
const surfaceClass =
  'absolute inset-0 flex flex-col items-center justify-center overflow-hidden bg-green-800 p-[var(--stage-margin)] text-center'

// The two message branches. Same size/weight treatment as every other
// stage's fallback copy, but ink-on-dark rather than text-text-secondary:
// those stages fall back onto a light ground and this one does not — the
// grey would sit at roughly 1.5:1 on green-800.
const messageClass = 'text-[length:var(--stage-heading)] font-heading text-ink-on-dark'

/**
 * The winner takeover (FR-18, UJ-2): the full-screen festive moment the
 * game ends on.
 *
 * The winner rule is NOT invented here. game/final.go's
 * ResultsForFinishedGame already defines it for the WhatsApp half of FR-18
 * — "every player whose Rank is 1 AND whose Score is strictly positive" —
 * and this file applies the identical predicate to snapshot.leaderboard,
 * the same RankLeaderboard output already on the wire. It never sorts and
 * never re-ranks: filtering by an already-assigned field is not the second
 * client-side ranking leaderboard-stage.tsx warns against, but a score
 * comparison or an `entry === leaderboard[0]` test would be.
 *
 * The strict positivity half is load bearing, not defensive: StopGame can
 * finish a game from question_open before any Reveal, at which point every
 * player sits at 0 and RankLeaderboard hands all of them rank 1. Without
 * it, an aborted game congratulates the entire room on winning with nothing.
 *
 * This stage publishes names and one score, which is its entire purpose —
 * unlike reveal-stage.tsx it has no per-participant privacy fence to build.
 * It still never reads snapshot.participants, so no phone number and no
 * per-question grade can reach it.
 */
export function WinnerStage({ snapshot, reducedMotion }: StageProps) {
  // NOT stage-props.ts's isDegradedFrame(), and this is the single most
  // important line in the file. That helper's second disjunct
  // (currentQuestion === null) is unconditionally TRUE at `finished` on
  // every REAL frame: games.sql's FinishGame resets
  // current_question_position to 0 on every transition into this state
  // (deliberately — a stale position would make buildSnapshot keep
  // reporting a currentQuestion for a game that already ended), and
  // buildSnapshot only populates `current` when the position is > 0.
  // Reusing the shared helper here would show the waiting copy on every
  // finished game, forever, and never render a winner.
  //
  // QuestionCount is never reset — it is ListQuestionsByGame's length
  // regardless of state or position — so it alone separates emptySnapshot's
  // degraded fallback (QuestionCount: 0, the Go zero value) from a real
  // finished frame (>= 1, because startGameNoQuestions' guard means no game
  // can start without at least one question). Do not "simplify" this back
  // to the shared helper. (Story 4.6, derived requirement 5.)
  if (snapshot.questionCount === 0) {
    return (
      <div className={surfaceClass}>
        <p className={messageClass}>{strings.display.waiting}</p>
      </div>
    )
  }

  const winners = snapshot.leaderboard.filter((entry) => entry.rank === 1 && entry.score > 0)
  // Destructured rather than tested with `winners.length === 0` — the two
  // are equivalent, but noUncheckedIndexedAccess is off, so winners[0]
  // types as LeaderboardEntry and would lie about an empty array. All tied
  // winners share one score by construction, so the first one carries it.
  const [topWinner] = winners

  if (!topWinner) {
    // A HEALTHY frame with nobody to congratulate — a different true thing
    // from the guard above, and it gets a different sentence. Two ways
    // here, and they collapse to one UI fact on purpose: the Organizer
    // stopped the game before any Reveal (everyone at 0, all rank 1, all
    // excluded by the positivity condition), or nobody registered at all
    // (nothing gates game start on a roster). messages_he.go's
    // msgFinalResultsNoWinner already treats the two identically for the
    // same reason, so this is not a third sentence to invent.
    // No confetti: there is nothing being celebrated.
    return (
      <div className={surfaceClass}>
        <p className={messageClass}>{strings.display.winner.noWinner}</p>
      </div>
    )
  }

  // Killed from the PROP, not from a media query — fourth stage in a row
  // with the same requirement (4.3, 4.4, 4.5, and now this one). A CSS-only
  // kill is invisible to jsdom so it could not be asserted, and the
  // Organizer's room-level toggle is not a media query at all.
  const animated = !reducedMotion

  return (
    <div className={surfaceClass}>
      {/* One aria-hidden wrapper rather than 22: the celebration is pure
          decoration and carries no information a screen reader needs. It
          also has to be the pieces' containing block, so their percentage
          coordinates resolve against the stage rather than against whatever
          the shell happens to be. Rendered BEFORE the content, as in the
          mockup, so it sits behind it in paint order.

          pointer-events-none is the mockup's fourth declaration on .confetti
          and was dropped in the first port (code review, 2026-08-14). The
          overlay is inset-0 across the whole stage, so without it the
          decoration swallows pointer events everywhere the z-10 content box
          does not cover. Latent while nothing here is interactive, which is
          exactly why it has to be restored now rather than debugged later. */}
      <div aria-hidden="true" className="pointer-events-none absolute inset-0 overflow-hidden">
        {confettiPieces.map((piece, index) => (
          <span
            // key={index} deliberately: the pieces are a fixed decorative
            // set with no identity, and position IS their identity — the
            // same call reveal-stage.tsx makes for its option rows.
            key={index}
            // Inline, per piece: these four values are DATA (the mockup's
            // own scatter), the same shape --stage-row-shift uses.
            style={{
              left: piece.left,
              top: piece.top,
              animationDuration: piece.duration,
              animationDelay: piece.delay,
            }}
            // `absolute` is unconditional and stage-confetti-piece is not:
            // the class carries the animation only, so that under reduced
            // motion every piece holds its scattered coordinates with no
            // transform — the mockup's own "static celebratory frame".
            className={`absolute ${colorClass[piece.color]} ${shapeClass[piece.shape]} ${animated ? 'stage-confetti-piece' : ''}`}
          />
        ))}
      </div>

      {/* z-10 over the confetti, which is positioned and would otherwise
          paint above this in-flow content.

          gap-[1.25vw] is the mockup's .w-name / .w-score margin-top
          (1.25cqw) under derived requirement 11's straight rename, NOT a
          recomputation onto the project's nearest fixed stop (code review,
          2026-08-14). The first port used gap-6 citing reveal-stage.tsx's
          precedent, but that precedent does not transfer: reveal-stage
          deviates from ITS mockup because its values are copied from the
          question stage, so that the question text does not jump at the
          moment of reveal. This stage shares no element with any other
          stage, so no continuity constraint exists — and a fixed 24px stop
          is the one measure here that would stop scaling with the projector
          (24px at 1280, where the mockup specifies 16px).

          max-w-full so the winner name below has a bounded line box to
          break inside; without it a flex item under items-center sizes to
          its own min-content and a long unbreakable name simply overflows. */}
      <div className="relative z-10 flex max-w-full flex-col items-center gap-[1.25vw]">
        {/* leading-heading (1.38) is the mockup's .w-tagline and .w-score
            line-height, transcribed here and on the score line below (code
            review, 2026-08-14). The first port gave the <h1> its
            leading-[1.1] from .w-name but left these two on preflight's
            1.5, so two of the three line-heights on this stage came from
            the pixel-authoritative mockup and one pair did not. The token
            already exists (--leading-heading), and reveal-stage.tsx uses
            it by the same name. */}
        <p className="text-[length:var(--stage-body)] font-ui leading-heading text-ink-on-dark-muted">
          {strings.display.winner.tagline}
        </p>

        {/* <h1>, closing deferred-work.md's heading-semantics entry rather
            than deferring it a fourth time (derived requirement 8). The
            winner name is this stage's one dominant text element — exactly
            the reasoning 4.3 applied to the question and 4.4 to the
            reveal's shared question — and Epic 4 has no story after this
            one to inherit the deferral. Tailwind preflight resets h1's
            font-size and font-weight to inherit, so there is no visual
            change from the element choice.

            font-display is the WEIGHT token (900), per this @theme's
            four-role naming; the visual scale is carried entirely by
            --stage-winner-name. */}
        <h1 className="max-w-full text-[length:var(--stage-winner-name)] font-display leading-[1.1] text-gold">
          {winners.slice(0, maxNames).map((winner) => (
            // <bdi className="block"> and not <div>: h1's content model is
            // phrasing content, so a div here would be invalid markup for
            // the sake of one display property, while <bdi> is phrasing
            // content and gives the identical stacked layout. Stacked one
            // per line, which is what DESIGN.md's "up to 3 names stacked"
            // asks for — a layout instruction, not a text join.
            // messages_he.go's joinNames composes the WhatsApp tie sentence
            // and is deliberately not the pattern here.
            //
            // <bdi> rather than a plain span, matching leaderboard-stage.tsx
            // on the IDENTICAL field (code review, 2026-08-14): a display
            // name is free-form participant-supplied text, uncapped TEXT in
            // the schema and accepted verbatim by parseRenameName, so it can
            // be Latin, mixed, or start with a neutral or a digit. Inside
            // this dir="rtl" document a leading neutral would otherwise
            // resolve to the paragraph level and jump to the far end —
            // "!David" painting as "David!".
            //
            // break-words for the same field's other hazard: at
            // --stage-winner-name (124.8px at 1920) an unbreakable name past
            // ~26 characters exceeds the content box, and surfaceClass's
            // text-center + overflow-hidden would clip it SYMMETRICALLY —
            // the room reading the middle of a name with no ellipsis and no
            // sign anything was cut. leaderboard-stage truncates and
            // reveal-stage breaks; a winner's name is the one thing on this
            // screen that must never be silently shortened, so it wraps.
            //
            // Real identity, matching leaderboard-stage's precedent: a
            // winner's own participantId, not the array index.
            <bdi key={winner.participantId} className="block max-w-full break-words">
              {winner.displayName}
            </bdi>
          ))}
        </h1>

        {/* Shown ONCE regardless of how many names are stacked above it
            (A16 / DESIGN.md) — outside the loop, not repeated per winner.
            Raw digits, no toLocaleString and no thousands separator,
            matching messages_he.go's bare strconv.Itoa and
            leaderboard-stage.tsx's own {entry.score}: the mockup's
            illustrative comma is the one thing on this screen the codebase
            disagrees with, and three existing sources agree with each other
            (derived requirement 7).

            A PLAIN <bdi>, not <bdi dir="ltr">: the composed value is a
            Hebrew sentence, so forcing LTR would mirror it and put the
            digits on the wrong side of the word. dir="auto" resolves RTL
            from the first strong character while still isolating the whole
            run from its surroundings, and the digit run keeps its own LTR
            order inside. */}
        <p className="text-[length:var(--stage-ui)] font-ui leading-heading text-ink-on-dark-muted tabular-nums">
          <bdi>{strings.display.winner.scoreSuffix(topWinner.score)}</bdi>
        </p>
      </div>
    </div>
  )
}
