// Theme and text size, kept per browser in localStorage. public/theme.js applies the same choice
// before first paint; this module changes it while the app runs.
export type ThemeChoice = 'system' | 'light' | 'dark'
export type SizeChoice = 'small' | 'default' | 'large'

export const THEME_KEY = 'mooring.theme'
export const SIZE_KEY = 'mooring.fontsize'
export const THEMES: ThemeChoice[] = ['system', 'light', 'dark']
export const SIZES: SizeChoice[] = ['small', 'default', 'large']

type Store = Pick<Storage, 'getItem' | 'setItem'>

export const browserStore = (): Store | null => {
  try {
    return localStorage
  } catch {
    return null
  }
}

function read<T extends string>(key: string, allowed: T[], fallback: T, store: Store | null): T {
  try {
    const v = store?.getItem(key)
    return allowed.find((a) => a === v) ?? fallback
  } catch {
    return fallback
  }
}

export const readTheme = (store: Store | null = browserStore()) => read(THEME_KEY, THEMES, 'system', store)
export const readSize = (store: Store | null = browserStore()) => read(SIZE_KEY, SIZES, 'default', store)

/** The theme that is drawn: "system" follows the browser's own setting. */
export const resolveTheme = (choice: ThemeChoice, systemDark: boolean): 'light' | 'dark' => (choice === 'system' ? (systemDark ? 'dark' : 'light') : choice)

export const oppositeTheme = (drawn: 'light' | 'dark'): 'light' | 'dark' => (drawn === 'dark' ? 'light' : 'dark')

export function write(key: string, value: string, store: Store | null) {
  try {
    store?.setItem(key, value)
  } catch {
    // Storage can be blocked or full; the choice then lasts until the page closes.
  }
}

/** Sets the data attributes the stylesheet reads, and tells the terminal to repaint its colours. */
export function applyAppearance(theme: ThemeChoice, size: SizeChoice, systemDark: boolean, root: HTMLElement = document.documentElement) {
  root.dataset.theme = resolveTheme(theme, systemDark)
  root.dataset.size = size
  window.dispatchEvent(new Event('mooring-theme'))
}
