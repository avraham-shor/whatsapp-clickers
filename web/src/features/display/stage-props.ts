import type { LobbySnapshot } from '@/lib/types'

/** What every stage component receives. A stage gets the whole snapshot
 * (not narrowed props) because the snapshot is the store — narrowing would
 * mean editing this shell every time a stage needs one more field.
 * reducedMotion is already the OR of the viewer's OS setting and the
 * Organizer's room-level setting: no stage should ever call matchMedia.
 *
 * Its own module, and not `@/lib/types` (whose header is "Wire types
 * mirroring the Go payloads" — this is a component contract, not a wire
 * type) and no longer display-page.tsx: that file value-imports every
 * stage, so the display module graph held a real cycle apart with one
 * `type` keyword that no type checker enforces. Writing `import { StageProps }`
 * in any stage file reintroduced it, and the symptom is an undefined
 * component at module-init time — a blank projector.
 * (deferred-work.md, 4.2 entry; closed by story 4.3.) */
export interface StageProps {
  snapshot: LobbySnapshot
  reducedMotion: boolean
}
