import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { Question, QuestionPackage, QuestionPackageList } from '@/lib/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

// The Question Bank browse surface (EXPERIENCE.md): import's only verb needs
// a target Game, so the bank lives in the editor's dialog — no standalone
// page. Copies are appended to the game; the refetched game query (with the
// new rows and their badges) is the visible confirmation.
export function PackageImportDialog({
  gameId,
  onClose,
}: {
  gameId: string
  onClose: () => void
}) {
  const queryClient = useQueryClient()

  const packages = useQuery({
    queryKey: ['question-packages'],
    queryFn: () => api<QuestionPackageList>('/api/question-packages'),
  })

  const importPackage = useMutation({
    mutationFn: (packageId: string) =>
      api<{ items: Question[] }>(
        `/api/games/${gameId}/questions/import-package`,
        {
          method: 'POST',
          body: JSON.stringify({ packageId }),
        },
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games', gameId] })
      queryClient.invalidateQueries({ queryKey: ['games'] })
      onClose()
    },
  })

  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-h-[85svh] overflow-y-auto rounded-xl">
        <DialogHeader>
          <DialogTitle className="font-heading text-host-text">
            {strings.questionBank.dialogTitle}
          </DialogTitle>
        </DialogHeader>

        {packages.isPending && (
          <p className="text-host-text-secondary">{strings.common.loading}</p>
        )}

        {packages.isError && (
          <p role="alert" className="text-sm text-error">
            {strings.questionBank.loadError}
          </p>
        )}

        {packages.isSuccess &&
          (packages.data.items.length === 0 ? (
            // An empty bank explains that packages are on the way — never a
            // bare list (AC-4; unreachable once seeded, still required).
            <div className="flex flex-col gap-2 rounded-md border border-host-border bg-surface-raised p-6">
              <p className="text-host-text">{strings.questionBank.emptyTitle}</p>
              <p className="text-host-text-secondary">
                {strings.questionBank.emptyBody}
              </p>
            </div>
          ) : (
            <ul className="flex flex-col gap-4">
              {packages.data.items.map((pkg) => (
                <PackageCard
                  key={pkg.id}
                  pkg={pkg}
                  importPending={importPackage.isPending}
                  onImport={() => importPackage.mutate(pkg.id)}
                />
              ))}
            </ul>
          ))}

        {importPackage.isError && (
          <p role="alert" className="text-sm text-error">
            {strings.questionBank.importError}
          </p>
        )}
      </DialogContent>
    </Dialog>
  )
}

// A13 package card: title, question count, first-question preview. Level-1
// surface — no shadow inside the dialog (the modal is the elevated one).
function PackageCard({
  pkg,
  importPending,
  onImport,
}: {
  pkg: QuestionPackage
  importPending: boolean
  onImport: () => void
}) {
  return (
    <li className="flex flex-col gap-3 rounded-md border border-host-border bg-surface-raised p-4">
      <div className="flex flex-col gap-1">
        <p className="text-host-text">{pkg.title}</p>
        <p className="text-sm text-host-text-secondary">
          {strings.questionBank.questionsCount(pkg.questionCount)}
        </p>
        {pkg.preview && (
          <p className="line-clamp-2 text-sm text-host-text-secondary">
            {pkg.preview}
          </p>
        )}
      </div>
      <Button
        variant="outline"
        className="h-10 self-start border-host-border text-host-text"
        disabled={importPending}
        onClick={onImport}
      >
        {strings.questionBank.importCta}
      </Button>
    </li>
  )
}
