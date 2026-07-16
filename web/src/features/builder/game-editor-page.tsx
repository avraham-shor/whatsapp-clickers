import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, ChevronUp } from 'lucide-react'

import { api, ApiError } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { Game, Question } from '@/lib/types'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { QuestionEditor } from '@/features/builder/question-editor'

type EditorState =
  | { mode: 'closed' }
  | { mode: 'create' }
  | { mode: 'edit'; question: Question }

export function GameEditorPage() {
  const { gameId = '' } = useParams()
  const queryClient = useQueryClient()
  const [editor, setEditor] = useState<EditorState>({ mode: 'closed' })

  const game = useQuery({
    queryKey: ['games', gameId],
    queryFn: () => api<Game>(`/api/games/${gameId}`),
  })

  const reorder = useMutation({
    mutationFn: (questionIds: string[]) =>
      api<void>(`/api/games/${gameId}/questions/reorder`, {
        method: 'POST',
        body: JSON.stringify({ questionIds }),
      }),
    // Optimistic order: apply locally, roll back on failure.
    onMutate: async (questionIds) => {
      await queryClient.cancelQueries({ queryKey: ['games', gameId] })
      const previous = queryClient.getQueryData<Game>(['games', gameId])
      if (previous) {
        const byId = new Map(previous.questions.map((q) => [q.id, q]))
        queryClient.setQueryData<Game>(['games', gameId], {
          ...previous,
          questions: questionIds.flatMap((id, index) => {
            const question = byId.get(id)
            return question ? [{ ...question, position: index + 1 }] : []
          }),
        })
      }
      return { previous }
    },
    onError: (_error, _ids, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['games', gameId], context.previous)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['games', gameId] })
    },
  })

  if (game.isPending) {
    return <p className="text-host-text-secondary">{strings.common.loading}</p>
  }
  if (game.isError) {
    if (game.error instanceof ApiError && game.error.status === 404) {
      return (
        <div className="flex flex-col items-start gap-4">
          <p className="text-host-text">{strings.gameEditor.notFound}</p>
          <Link to="/" className="text-green-800 underline">
            {strings.gameEditor.backToGames}
          </Link>
        </div>
      )
    }
    return (
      <p role="alert" className="text-host-text">
        {strings.common.connectionError}
      </p>
    )
  }

  const { questions } = game.data

  const moveQuestion = (index: number, direction: -1 | 1) => {
    const ids = questions.map((question) => question.id)
    const target = index + direction
    if (target < 0 || target >= ids.length) return
    ;[ids[index], ids[target]] = [ids[target], ids[index]]
    reorder.mutate(ids)
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-heading text-host-text">
            {game.data.title}
          </h1>
          <p className="text-host-text-secondary">
            {strings.gameEditor.joinCodeLabel}{' '}
            {/* Latin/digit LTR token inside RTL — bidi-isolate (DESIGN.md). */}
            <bdi
              dir="ltr"
              className="text-lg font-heading tracking-wide text-host-text"
            >
              {game.data.joinCode}
            </bdi>
          </p>
        </div>
        <Button
          onClick={() => setEditor({ mode: 'create' })}
          className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
        >
          {strings.gameEditor.addQuestion}
        </Button>
      </header>

      {reorder.isError && (
        <p role="alert" className="text-sm text-error">
          {strings.gameEditor.reorderError}
        </p>
      )}

      {questions.length === 0 ? (
        <div className="flex flex-col items-start gap-2 rounded-md border border-host-border bg-surface-raised p-6">
          <p className="text-host-text">{strings.gameEditor.emptyStateTitle}</p>
          <p className="text-host-text-secondary">
            {strings.gameEditor.emptyStateBody}
          </p>
        </div>
      ) : (
        <ol className="flex flex-col gap-4">
          {questions.map((question, index) => (
            <QuestionRow
              key={question.id}
              gameId={gameId}
              question={question}
              isFirst={index === 0}
              isLast={index === questions.length - 1}
              reorderPending={reorder.isPending}
              onMoveUp={() => moveQuestion(index, -1)}
              onMoveDown={() => moveQuestion(index, 1)}
              onEdit={() => setEditor({ mode: 'edit', question })}
            />
          ))}
        </ol>
      )}

      {editor.mode !== 'closed' && (
        <QuestionEditor
          gameId={gameId}
          question={editor.mode === 'edit' ? editor.question : undefined}
          onClose={() => setEditor({ mode: 'closed' })}
        />
      )}
    </div>
  )
}

function QuestionRow({
  gameId,
  question,
  isFirst,
  isLast,
  reorderPending,
  onMoveUp,
  onMoveDown,
  onEdit,
}: {
  gameId: string
  question: Question
  isFirst: boolean
  isLast: boolean
  reorderPending: boolean
  onMoveUp: () => void
  onMoveDown: () => void
  onEdit: () => void
}) {
  const queryClient = useQueryClient()

  const deleteQuestion = useMutation({
    mutationFn: () =>
      api<void>(`/api/games/${gameId}/questions/${question.id}`, {
        method: 'DELETE',
      }),
    // Reconcile on both success and error: a 404 (question already deleted in
    // another tab) must refetch so the phantom row — and its error banner —
    // clear instead of sticking forever.
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['games', gameId] })
      queryClient.invalidateQueries({ queryKey: ['games'] })
    },
  })

  const typeLabel =
    question.type === 'mcq'
      ? strings.gameEditor.typeMcq
      : strings.gameEditor.typeFreeText

  return (
    <li className="flex flex-col gap-3 rounded-md border border-host-border bg-surface-raised p-4">
      <div className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3 text-sm">
            <span className="rounded-sm border border-host-border px-2 py-1 text-host-text-secondary">
              {typeLabel}
            </span>
            <span className="text-host-text-secondary">
              {strings.gameEditor.timeLimitSeconds(question.timeLimitSeconds)}
            </span>
          </div>
          <p className="text-host-text">{question.text}</p>
          {question.type === 'mcq' && question.correctOption ? (
            <p className="text-sm text-host-text-secondary">
              {strings.gameEditor.correctOption(
                strings.questionEditor.optionLetters[question.correctOption - 1],
              )}
            </p>
          ) : (
            <p className="text-sm text-host-text-secondary">
              {strings.gameEditor.acceptedAnswersCount(
                question.acceptedAnswers?.length ?? 0,
              )}
            </p>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Button
            variant="outline"
            className="h-10 w-10 border-host-border p-0 text-host-text"
            aria-label={strings.gameEditor.moveUp}
            disabled={isFirst || reorderPending}
            onClick={onMoveUp}
          >
            <ChevronUp aria-hidden />
          </Button>
          <Button
            variant="outline"
            className="h-10 w-10 border-host-border p-0 text-host-text"
            aria-label={strings.gameEditor.moveDown}
            disabled={isLast || reorderPending}
            onClick={onMoveDown}
          >
            <ChevronDown aria-hidden />
          </Button>
        </div>
      </div>

      {deleteQuestion.isError && (
        <p role="alert" className="text-sm text-error">
          {strings.gameEditor.deleteError}
        </p>
      )}

      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          className="h-10 border-host-border text-host-text"
          onClick={onEdit}
        >
          {strings.gameEditor.editQuestion}
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              variant="outline"
              className="h-10 border-host-border text-error"
              disabled={deleteQuestion.isPending}
            >
              {strings.gameEditor.deleteQuestion}
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent className="rounded-xl">
            <AlertDialogHeader>
              <AlertDialogTitle className="font-heading text-host-text">
                {strings.gameEditor.deleteConfirmTitle}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {strings.gameEditor.deleteConfirmBody}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel className="h-10">
                {strings.common.cancel}
              </AlertDialogCancel>
              <AlertDialogAction
                className="h-10 bg-error text-ink-on-dark hover:bg-error/90"
                onClick={() => deleteQuestion.mutate()}
              >
                {strings.gameEditor.deleteConfirm}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </li>
  )
}
