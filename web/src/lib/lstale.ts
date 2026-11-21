// CodeMirror's completion plugin accepts a server answer on a timer. If the editor was reconfigured
// or replaced in the meantime (the tab closed, the connection ended) the plugin is gone with its
// state field, and the timer still fires and throws "Field is not present in this state". Nothing
// is lost: that answer is not wanted any more. Only that error is dropped; others still reach the console.

/** True for the "field is not present" error that the completion timer throws after its editor changed. */
export function isLateFieldError(error: unknown): boolean {
  return error instanceof RangeError && error.message === 'Field is not present in this state'
}

let installed = false

/** Called when a language server connection is made; installs the handler once. */
export function ignoreLateCompletions(target: Pick<Window, 'addEventListener'> = window): void {
  if (installed) return
  installed = true
  target.addEventListener('error', (e) => {
    if (isLateFieldError((e as ErrorEvent).error)) e.preventDefault()
  })
}
