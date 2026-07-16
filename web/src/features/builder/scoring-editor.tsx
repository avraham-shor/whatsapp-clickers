import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { Game } from '@/lib/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

interface ScoringBody {
  pointsPerCorrect: number
  speedBonusFirst: number
  speedBonusSecond: number
  speedBonusThird: number
}

const SCORING_MIN = 0
const SCORING_MAX = 10000

// Full-replacement PUT: the form always sends all four values. Pre-fill
// comes from the server (schema defaults) — no client-side constants.
export function ScoringEditor({ game }: { game: Game }) {
  const queryClient = useQueryClient()

  const [points, setPoints] = useState(String(game.pointsPerCorrect))
  const [first, setFirst] = useState(String(game.speedBonusFirst))
  const [second, setSecond] = useState(String(game.speedBonusSecond))
  const [third, setThird] = useState(String(game.speedBonusThird))
  const [validationMessage, setValidationMessage] = useState<string>()

  const save = useMutation({
    mutationFn: (body: ScoringBody) =>
      api<ScoringBody>(`/api/games/${game.id}/scoring`, {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games', game.id] })
      queryClient.invalidateQueries({ queryKey: ['games'] })
    },
  })

  // Any edit clears a stale "נשמר ✓" and a stale validation message, so
  // neither lingers over changed (possibly corrected) values.
  const edit = (set: (value: string) => void) => (value: string) => {
    save.reset()
    setValidationMessage(undefined)
    set(value)
  }

  const submit = () => {
    const raw = [points, first, second, third]
    const values = raw.map(Number)
    if (
      // A blank field must be an error, not a value: Number('') === 0 and 0
      // is meaningful (a disabled bonus) — never submit it by accident.
      raw.some((text) => text.trim() === '') ||
      values.some(
        (value) =>
          !Number.isInteger(value) || value < SCORING_MIN || value > SCORING_MAX,
      )
    ) {
      setValidationMessage(strings.scoring.validation)
      return
    }
    setValidationMessage(undefined)
    const [pointsPerCorrect, speedBonusFirst, speedBonusSecond, speedBonusThird] =
      values
    save.mutate({
      pointsPerCorrect,
      speedBonusFirst,
      speedBonusSecond,
      speedBonusThird,
    })
  }

  const errorText =
    validationMessage ?? (save.isError ? strings.scoring.errorServer : undefined)

  return (
    <section
      aria-labelledby="scoring-title"
      className="flex flex-col gap-4 rounded-md border border-host-border bg-surface-raised p-6"
    >
      <h2 id="scoring-title" className="text-lg font-heading text-host-text">
        {strings.scoring.title}
      </h2>
      <form
        className="flex flex-col gap-6"
        onSubmit={(event) => {
          event.preventDefault()
          submit()
        }}
      >
        <div className="flex flex-col gap-2">
          <Label htmlFor="scoring-points" className="text-host-text">
            {strings.scoring.pointsPerCorrectLabel}
          </Label>
          <Input
            id="scoring-points"
            type="number"
            min={SCORING_MIN}
            max={SCORING_MAX}
            step={1}
            required
            className="h-10 w-32"
            value={points}
            onChange={(event) => edit(setPoints)(event.target.value)}
            // Field-level error association: announced on focus (UX-DR12).
            aria-describedby={errorText ? 'scoring-error' : undefined}
          />
        </div>

        <fieldset className="flex flex-col gap-3">
          <legend className="text-sm text-host-text">
            {strings.scoring.bonusLegend}
          </legend>
          <p
            id="scoring-bonus-hint"
            className="text-sm text-host-text-secondary"
          >
            {strings.scoring.bonusHint}
          </p>
          <div className="flex flex-wrap gap-4">
            {(
              [
                ['scoring-bonus-first', strings.scoring.bonusFirstLabel, first, setFirst],
                ['scoring-bonus-second', strings.scoring.bonusSecondLabel, second, setSecond],
                ['scoring-bonus-third', strings.scoring.bonusThirdLabel, third, setThird],
              ] as const
            ).map(([id, label, value, set]) => (
              <div key={id} className="flex flex-col gap-2">
                <Label htmlFor={id} className="text-host-text">
                  {label}
                </Label>
                <Input
                  id={id}
                  type="number"
                  min={SCORING_MIN}
                  max={SCORING_MAX}
                  step={1}
                  required
                  className="h-10 w-32"
                  value={value}
                  onChange={(event) => edit(set)(event.target.value)}
                  aria-describedby={
                    errorText
                      ? 'scoring-bonus-hint scoring-error'
                      : 'scoring-bonus-hint'
                  }
                />
              </div>
            ))}
          </div>
        </fieldset>

        {errorText && (
          <p id="scoring-error" role="alert" className="text-sm text-error">
            {errorText}
          </p>
        )}

        <div className="flex items-center gap-3">
          <Button
            type="submit"
            disabled={save.isPending}
            className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
          >
            {strings.scoring.save}
          </Button>
          {/* polite live region: announced without interrupting (UX-DR14). */}
          <p aria-live="polite" className="text-sm text-success">
            {save.isSuccess ? strings.scoring.saved : ''}
          </p>
        </div>
      </form>
    </section>
  )
}
