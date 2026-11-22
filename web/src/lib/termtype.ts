// Opens a terminal and types a command into it without running it, so the user reads it and presses Enter.
import { actions } from './actions'
import { terms } from './termstore.svelte'

/** Waits for the new terminal's view, then types `text` (no Enter). Gives up after 10 seconds. */
export function terminalTyping(text: string): void {
  const before = new Set(terms.views.keys())
  actions.newTerminal()
  const stop = Date.now() + 10_000
  const timer = setInterval(() => {
    const fresh = [...terms.views.keys()].find((k) => !before.has(k))
    const view = fresh ? terms.views.get(fresh) : undefined
    const ui = fresh ? terms.ui[fresh] : undefined
    if (view && ui?.wire === 'live') {
      clearInterval(timer)
      view.press(text)
    } else if (Date.now() > stop) clearInterval(timer)
  }, 150)
}
