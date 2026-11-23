// A request from the sidebar to show one file's diff in the Changes view. The view takes it once.
class ChangesPick {
  path = $state('')
  staged = $state(false)
  /** Counts requests, so asking for the same file twice still reaches a view that is already open. */
  n = $state(0)

  ask(path: string, staged: boolean) {
    this.path = path
    this.staged = staged
    this.n++
  }

  /** Returns the pending request and clears it. */
  take(): { path: string; staged: boolean } | null {
    if (!this.path) return null
    const r = { path: this.path, staged: this.staged }
    this.path = ''
    return r
  }
}

export const changesPick = new ChangesPick()
