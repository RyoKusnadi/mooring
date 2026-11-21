import { SIZE_KEY, THEME_KEY, applyAppearance, browserStore, oppositeTheme, readSize, readTheme, resolveTheme, write, type SizeChoice, type ThemeChoice } from './appearance'

class Appearance {
  theme = $state<ThemeChoice>(readTheme())
  size = $state<SizeChoice>(readSize())
  private media = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : null
  private systemDark = $state(!!this.media?.matches)
  /** The theme on screen now: light or dark. */
  drawn = $derived(resolveTheme(this.theme, this.systemDark))

  constructor() {
    this.media?.addEventListener('change', (e) => {
      this.systemDark = e.matches
      if (this.theme === 'system') this.apply()
    })
  }

  private apply() {
    applyAppearance(this.theme, this.size, this.systemDark)
  }

  setTheme(t: ThemeChoice) {
    this.theme = t
    write(THEME_KEY, t, browserStore())
    this.apply()
  }

  /** The quick switch: draws the other theme and keeps it, even when the choice was "system". */
  toggleTheme() {
    this.setTheme(oppositeTheme(this.drawn))
  }

  setSize(s: SizeChoice) {
    this.size = s
    write(SIZE_KEY, s, browserStore())
    this.apply()
  }
}

export const appearance = new Appearance()
