import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'
import { strings } from '@/lib/strings.he'
import type { Question, QuestionType } from '@/lib/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'

// A8: the field pre-fills 20s and carries visible guidance that 30–45s
// accommodates screen-reader users and slower-motor participants.
const DEFAULT_TIME_LIMIT = 20

interface QuestionBody {
  type: QuestionType
  text: string
  timeLimitSeconds: number
  options?: string[]
  correctOption?: number
  acceptedAnswers?: string[]
}

// One component for create + edit (dialog — modals are the one
// shadow-allowed surface). Type is picked at creation and immutable after.
export function QuestionEditor({
  gameId,
  question,
  onClose,
}: {
  gameId: string
  question?: Question
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const isEdit = question !== undefined

  const [type, setType] = useState<QuestionType>(question?.type ?? 'mcq')
  const [text, setText] = useState(question?.text ?? '')
  const [options, setOptions] = useState<string[]>(
    question?.options ?? ['', '', '', ''],
  )
  const [correctOption, setCorrectOption] = useState(
    question?.correctOption ?? 0,
  )
  const [answers, setAnswers] = useState<string[]>(
    question?.acceptedAnswers ?? [''],
  )
  const [timeLimit, setTimeLimit] = useState(
    String(question?.timeLimitSeconds ?? DEFAULT_TIME_LIMIT),
  )
  const [validationMessage, setValidationMessage] = useState<string>()

  const save = useMutation({
    mutationFn: (body: QuestionBody) =>
      isEdit
        ? api<Question>(`/api/games/${gameId}/questions/${question.id}`, {
            method: 'PUT',
            body: JSON.stringify(body),
          })
        : api<Question>(`/api/games/${gameId}/questions`, {
            method: 'POST',
            body: JSON.stringify(body),
          }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games', gameId] })
      queryClient.invalidateQueries({ queryKey: ['games'] })
      onClose()
    },
  })

  // Client-side mirror of the server boundary rules (trim first).
  const submit = () => {
    const trimmedText = text.trim()
    if (trimmedText.length < 1 || trimmedText.length > 500) {
      setValidationMessage(strings.questionEditor.validationText)
      return
    }
    const limit = Number(timeLimit)
    if (!Number.isInteger(limit) || limit < 5 || limit > 300) {
      setValidationMessage(strings.questionEditor.validationTimeLimit)
      return
    }
    const body: QuestionBody = {
      type,
      text: trimmedText,
      timeLimitSeconds: limit,
    }
    if (type === 'mcq') {
      const trimmedOptions = options.map((option) => option.trim())
      if (
        trimmedOptions.some(
          (option) => option.length < 1 || option.length > 200,
        )
      ) {
        setValidationMessage(strings.questionEditor.validationOptions)
        return
      }
      if (correctOption < 1 || correctOption > 4) {
        setValidationMessage(strings.questionEditor.validationCorrect)
        return
      }
      body.options = trimmedOptions
      body.correctOption = correctOption
    } else {
      // Empty rows are dropped; what remains must be 1–20 valid answers.
      const trimmedAnswers = answers
        .map((answer) => answer.trim())
        .filter((answer) => answer.length > 0)
      if (
        trimmedAnswers.length < 1 ||
        trimmedAnswers.length > 20 ||
        trimmedAnswers.some((answer) => answer.length > 200)
      ) {
        setValidationMessage(strings.questionEditor.validationAnswers)
        return
      }
      body.acceptedAnswers = trimmedAnswers
    }
    setValidationMessage(undefined)
    save.mutate(body)
  }

  const errorText =
    validationMessage ??
    (save.isError ? strings.questionEditor.errorServer : undefined)

  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="max-h-[85svh] overflow-y-auto rounded-xl">
        <DialogHeader>
          <DialogTitle className="font-heading text-host-text">
            {isEdit
              ? strings.questionEditor.editTitle
              : strings.questionEditor.createTitle}
          </DialogTitle>
        </DialogHeader>
        <form
          className="flex flex-col gap-6"
          aria-describedby={errorText ? 'question-editor-error' : undefined}
          onSubmit={(event) => {
            event.preventDefault()
            submit()
          }}
        >
          {!isEdit && (
            <fieldset className="flex flex-col gap-2">
              <legend className="text-sm text-host-text">
                {strings.questionEditor.typeLabel}
              </legend>
              <RadioGroup
                value={type}
                onValueChange={(next) => setType(next as QuestionType)}
                className="flex flex-row gap-6"
              >
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="mcq" id="question-type-mcq" />
                  <Label htmlFor="question-type-mcq" className="text-host-text">
                    {strings.questionEditor.typeMcq}
                  </Label>
                </div>
                <div className="flex items-center gap-2">
                  <RadioGroupItem value="free_text" id="question-type-free" />
                  <Label htmlFor="question-type-free" className="text-host-text">
                    {strings.questionEditor.typeFreeText}
                  </Label>
                </div>
              </RadioGroup>
            </fieldset>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="question-text" className="text-host-text">
              {strings.questionEditor.textLabel}
            </Label>
            <Textarea
              id="question-text"
              required
              value={text}
              onChange={(event) => setText(event.target.value)}
            />
          </div>

          {type === 'mcq' ? (
            <fieldset className="flex flex-col gap-3">
              <legend className="text-sm text-host-text">
                {strings.questionEditor.correctLabel}
              </legend>
              {/* Letters are presentation by position — never stored. */}
              <RadioGroup
                value={correctOption > 0 ? String(correctOption) : ''}
                onValueChange={(next) => setCorrectOption(Number(next))}
                className="flex flex-col gap-3"
              >
                {strings.questionEditor.optionLetters.map((letter, index) => (
                  <div key={letter} className="flex items-center gap-3">
                    <RadioGroupItem
                      value={String(index + 1)}
                      id={`question-correct-${index}`}
                      aria-label={strings.questionEditor.markCorrect(letter)}
                    />
                    <Label
                      htmlFor={`question-option-${index}`}
                      className="w-16 shrink-0 text-host-text"
                    >
                      {strings.questionEditor.optionLabel(letter)}
                    </Label>
                    <Input
                      id={`question-option-${index}`}
                      className="h-10"
                      value={options[index]}
                      onChange={(event) =>
                        setOptions((current) =>
                          current.map((option, i) =>
                            i === index ? event.target.value : option,
                          ),
                        )
                      }
                    />
                  </div>
                ))}
              </RadioGroup>
            </fieldset>
          ) : (
            <fieldset className="flex flex-col gap-3">
              <legend className="text-sm text-host-text">
                {strings.questionEditor.acceptedAnswersLabel}
              </legend>
              <p className="text-sm text-host-text-secondary">
                {strings.questionEditor.acceptedAnswersHint}
              </p>
              {answers.map((answer, index) => (
                <div key={index} className="flex items-center gap-3">
                  <Label
                    htmlFor={`question-answer-${index}`}
                    className="w-24 shrink-0 text-host-text"
                  >
                    {strings.questionEditor.answerRowLabel(index + 1)}
                  </Label>
                  <Input
                    id={`question-answer-${index}`}
                    className="h-10"
                    value={answer}
                    onChange={(event) =>
                      setAnswers((current) =>
                        current.map((a, i) =>
                          i === index ? event.target.value : a,
                        ),
                      )
                    }
                  />
                  <Button
                    type="button"
                    variant="outline"
                    className="h-10 shrink-0 border-host-border text-host-text"
                    disabled={answers.length === 1}
                    onClick={() =>
                      setAnswers((current) =>
                        current.filter((_, i) => i !== index),
                      )
                    }
                  >
                    {strings.questionEditor.removeAnswer(index + 1)}
                  </Button>
                </div>
              ))}
              <Button
                type="button"
                variant="outline"
                className="h-10 self-start border-host-border text-host-text"
                disabled={answers.length >= 20}
                onClick={() => setAnswers((current) => [...current, ''])}
              >
                {strings.questionEditor.addAnswer}
              </Button>
            </fieldset>
          )}

          <div className="flex flex-col gap-2">
            <Label htmlFor="question-time-limit" className="text-host-text">
              {strings.questionEditor.timeLimitLabel}
            </Label>
            <Input
              id="question-time-limit"
              type="number"
              min={5}
              max={300}
              step={1}
              required
              className="h-10 w-32"
              value={timeLimit}
              onChange={(event) => setTimeLimit(event.target.value)}
              aria-describedby="question-time-limit-hint"
            />
            <p
              id="question-time-limit-hint"
              className="text-sm text-host-text-secondary"
            >
              {strings.questionEditor.timeLimitHint}
            </p>
          </div>

          {errorText && (
            <p
              id="question-editor-error"
              role="alert"
              className="text-sm text-error"
            >
              {errorText}
            </p>
          )}

          <div className="flex items-center justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              className="h-10 border-host-border text-host-text"
              onClick={onClose}
            >
              {strings.common.cancel}
            </Button>
            <Button
              type="submit"
              disabled={save.isPending}
              className="h-10 bg-green-800 text-ink-on-dark hover:bg-green-900"
            >
              {isEdit
                ? strings.questionEditor.submitEdit
                : strings.questionEditor.submitCreate}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
