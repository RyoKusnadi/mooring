// The Tab hint shows the first time an editor takes focus, then never again.
const HINT_KEY = 'mooring.tabhint'

class TabHint {
  shown = $state(false)

  on() {
    try {
      if (!this.shown && !localStorage.getItem(HINT_KEY)) this.shown = true
    } catch {
      // Storage is blocked: the hint simply does not show.
    }
  }

  off() {
    if (!this.shown) return
    this.shown = false
    try {
      localStorage.setItem(HINT_KEY, '1')
    } catch {
      // Not remembered; it shows again next time.
    }
  }
}

export const tabHint = new TabHint()
