import type { ComponentType } from 'react'

import { strings } from '@/lib/strings.he'
import type { StageProps } from './stage-props'

// Every state maps here until its own story (4.2–4.6) replaces that one
// entry in display-page's stageByState map; `draft` keeps it permanently
// (the launch CTA only appears from lobby onward, so a draft display is
// only ever reached by a hand-typed URL).
//
// Typed as ComponentType<StageProps> rather than declaring the props it
// does not read: it must stay assignable to the switcher's map, and an
// unused parameter is a lint error here.
//
// text-text-secondary on surface-base is 10.6:1 per DESIGN.md's contrast
// table — not ink-on-dark-muted, which is for the dark green hero band.
export const StagePlaceholder: ComponentType<StageProps> = () => (
  <p className="text-[length:var(--stage-heading)] font-heading text-text-secondary">
    {strings.display.waiting}
  </p>
)
