import { Link, useParams } from 'react-router'

import { strings } from '@/lib/strings.he'
import { ResultsSummary } from '@/features/results/results-summary'

// The durable post-game surface (FR-14, epic AC-2): a real URL under
// RequireAuth, so a direct link, a reload, or a fresh sign-in tomorrow all
// render the same summary — none of it depends on the WS session that ran
// the game. The SPA fallback already serves this path (spaHandler returns
// index.html for any extension-less non-reserved path), so no server-side
// routing change is needed.
export function ResultsPage() {
  const { gameId = '' } = useParams()
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <h1 className="text-2xl font-heading text-host-text">{strings.results.title}</h1>
      <ResultsSummary gameId={gameId} />
      <Link to="/" className="text-green-800 underline">
        {strings.gameEditor.backToGames}
      </Link>
    </div>
  )
}
