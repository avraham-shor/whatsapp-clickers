import { useEffect } from 'react'

/**
 * Fires onFire on a bare Space keydown when focus is on document.body — no
 * dialog or input focused, and a focused button already handles its own
 * native Space activation so this must not also fire in that case (no
 * global handler racing them, per AC-4). Shared by every "one persistent
 * button" primary-action surface (the live control panel, the lobby's
 * start button) so both get the same fix in one place.
 *
 * Ignores OS key auto-repeat (event.repeat) — without this, holding Space
 * re-fires onFire at the OS repeat rate, which can chain through several
 * state transitions from a single press.
 */
export function useSpaceAction(onFire: () => void, enabled: boolean) {
  useEffect(() => {
    if (!enabled) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.code !== 'Space' || event.repeat) return
      if (document.activeElement !== document.body) return
      event.preventDefault()
      onFire()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onFire, enabled])
}
