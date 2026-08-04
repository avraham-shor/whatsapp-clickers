import { strings } from '@/lib/strings.he'

interface ResponseStatsProps {
  count: number
}

// host-stat-pill (DESIGN.md): green-50 background, green-800 text,
// border-light border, pill shape. aria-live="polite" per UX-DR14 (score/
// count updates) — no throttling here: EXPERIENCE.md's throttling note
// (line 236) is written for the projected Audience Display (Epic 4), not
// this internal host-facing panel.
export function ResponseStats({ count }: ResponseStatsProps) {
  return (
    <span
      aria-live="polite"
      className="rounded-full border border-border-light bg-green-50 px-3 py-1 text-sm text-green-800"
    >
      {strings.live.answeredStat(count)}
    </span>
  )
}
