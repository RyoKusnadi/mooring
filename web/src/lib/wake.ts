// The network came back, or the tab was shown again. Sockets waiting out a backoff
// (up to 10 s) try at once instead, so the app is live as soon as it can be.
const fns = new Set<() => void>()
let bound = false

function run() {
  for (const fn of [...fns]) fn()
}

/** Adds a listener and returns the function that removes it. */
export function listen(target: EventTarget, type: string, fn: () => void): () => void {
  target.addEventListener(type, fn)
  return () => target.removeEventListener(type, fn)
}

/** Calls fn on every wake until the returned function is called. */
export function onWake(fn: () => void): () => void {
  if (!bound && typeof window !== 'undefined') {
    bound = true
    window.addEventListener('online', run)
    document.addEventListener('visibilitychange', () => {
      if (!document.hidden) run()
    })
  }
  fns.add(fn)
  return () => {
    fns.delete(fn)
  }
}
