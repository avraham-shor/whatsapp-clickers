import { strings } from '@/lib/strings.he'

function App() {
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-3 bg-surface-base">
      <h1 className="text-4xl font-heading text-text-primary">
        {strings.appTitle}
      </h1>
      <p className="text-lg font-body text-text-secondary">
        {strings.appTagline}
      </p>
    </main>
  )
}

export default App
