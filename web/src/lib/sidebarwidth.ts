// The sidebar's width: dragged or set with the arrow keys, remembered per browser.
export const SIDEBAR_DEFAULT = 264
export const SIDEBAR_MIN = 220
export const SIDEBAR_MAX = 480
export const SIDEBAR_KEY = 'mooring.sidebar'

type Store = Pick<Storage, 'getItem' | 'setItem'>

export const clampWidth = (w: number): number => (Number.isFinite(w) ? Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, Math.round(w))) : SIDEBAR_DEFAULT)

/** The widest sidebar this window allows: the main column keeps at least 560px, but never less than the minimum sidebar. */
export const MAIN_MIN = 560
export const fitWidth = (w: number, windowWidth: number): number => Math.max(SIDEBAR_MIN, Math.min(clampWidth(w), windowWidth - MAIN_MIN))

export function readWidth(store: Store | null): number {
  try {
    const v = store?.getItem(SIDEBAR_KEY)
    return v ? clampWidth(Number(v)) : SIDEBAR_DEFAULT
  } catch {
    return SIDEBAR_DEFAULT
  }
}

export function writeWidth(store: Store | null, w: number) {
  try {
    store?.setItem(SIDEBAR_KEY, String(clampWidth(w)))
  } catch {
    // Storage blocked: the width lasts until the page closes.
  }
}

export const browserStore = (): Store | null => {
  try {
    return localStorage
  } catch {
    return null
  }
}
