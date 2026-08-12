import { strings } from '@/lib/strings.he'

export type StageOptionVariant = 'default' | 'correct' | 'dimmed'

// COMPLETE class strings, never composed - Tailwind v4's scanner reads
// source text, so `bg-${x}` generates nothing and the pill would render
// unstyled on a projector with a green build.
const variantClass: Record<StageOptionVariant, string> = {
  default: 'bg-green-800 text-ink-on-dark',
  correct: 'bg-success text-ink-on-dark',
  dimmed: 'bg-green-100 text-text-primary',
}

// Everything the three variants share. The SIZING is deliberately absent
// and supplied by the caller: the question stage's full-width shrinking row
// and the reveal row's fixed 44% column are different layouts around the
// same pill.
//
// rounded-sm (8px) — "options are choices, not tags", never rounded-full.
// (Carried over from question-stage.tsx with the rest of the pill; it was
// dropped in the extraction and restored at code review, 2026-08-12. It
// earns its place more here than it did there: the reveal stage puts a
// rounded-full bar track on the SAME ROW as this pill, so the one rule the
// comment exists to protect now has a counter-example eight columns away.)
const baseClass = 'flex items-center gap-4 rounded-sm px-6 text-[length:var(--stage-body)] leading-body'

interface StageOptionProps {
  /** Already formatted by strings.display.question.optionLetter. */
  letter: string
  text: string
  variant?: StageOptionVariant
  /** Flex sizing, supplied by the caller: the question stage's
   *  full-width shrinking row vs the reveal row's fixed 44% column. */
  className?: string
}

/**
 * One answer option, in the three treatments DESIGN.md's frontmatter names
 * as separate components — `stage-option`, `stage-option-correct`,
 * `stage-option-dimmed` — which differ only in fill and text colour. One
 * component with a variant maps onto that 1:1; two copies of the class list
 * would let the letter/text weight rule, the 8px radius and the 40px size
 * drift apart between the question stage and the reveal stage.
 *
 * "Dim the fill, never the text" (DESIGN.md Don'ts): `dimmed` changes the
 * fill and the text COLOUR and never uses opacity-*. text-primary on
 * green-100 is 9.7:1.
 */
export function StageOption({ letter, text, variant = 'default', className = '' }: StageOptionProps) {
  return (
    // Joined with a template string, NOT cn(). cn is twMerge(clsx(...)) and
    // is used nowhere outside components/ui; twMerge silently dropping one of
    // two classes it thinks conflict is a failure whose only symptom is a
    // wrong-looking projector. The variant classes and the callers' sizing
    // classes cannot collide, so there is nothing to merge.
    <div className={`${baseClass} ${variantClass[variant]} ${className}`}>
      {/* Letter bold, text regular, in EVERY variant — DESIGN.md
          stage-option-dimmed keeps letterWeight 700 explicitly. DESIGN.md
          stage-option says letterWeight 700 / textWeight 500, and the Don'ts
          row calls same-weight "the letter becomes invisible". This
          project's @theme has no 700 step; font-heading (800) is the nearest
          existing one and preserves the CONTRAST the spec is about —
          recorded rather than solved by inventing a sixth weight token.
          The letter sits on the inline-start side for free: flex + gap-4 in
          a dir="rtl" document. No flex-row-reverse and no physical
          margins — logical properties only. */}
      <span className="font-heading">{letter}</span>
      <span className="font-body">{text}</span>
      {variant === 'correct' && (
        <>
          {/* ms-auto = margin-inline-start: auto, DESIGN.md
              stage-option-correct.icon's "✓ inline-end" verbatim. No
              physical margin utilities. The glyph is aria-hidden and the
              accessible name sits beside it: UX-DR14 requires the ✓ to be
              aria-labeled, and a bare "✓" read aloud is not a label. */}
          <span aria-hidden className="ms-auto font-heading">
            {strings.display.reveal.correctMark}
          </span>
          <span className="sr-only">{strings.display.reveal.correctOptionLabel}</span>
        </>
      )}
    </div>
  )
}
