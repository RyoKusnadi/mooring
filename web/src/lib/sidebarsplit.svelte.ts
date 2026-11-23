// The Chats block of the sidebar: open or closed, and how tall. Both are remembered per browser.
import { browserStore } from './sidebarwidth'

export const CHATS_MIN = 96
export const CHATS_MAX = 640
export const CHATS_STEP = 16
const OPEN_KEY = 'mooring.chats.open'
const HEIGHT_KEY = 'mooring.chats.height'

export const clampHeight = (h: number): number => Math.min(CHATS_MAX, Math.max(CHATS_MIN, Math.round(h)))

function read<T>(key: string, parse: (v: string) => T, fallback: T): T {
  try {
    const v = browserStore()?.getItem(key)
    return v == null ? fallback : parse(v)
  } catch {
    return fallback
  }
}

function write(key: string, value: string) {
  try {
    browserStore()?.setItem(key, value)
  } catch {
    // Storage blocked: the choice lasts until the page closes.
  }
}

class SidebarSplit {
  open = $state(read(OPEN_KEY, (v) => v !== '0', true))
  /** Pixels, or null for the default: as tall as the list, up to 40% of the sidebar. */
  height = $state<number | null>(read(HEIGHT_KEY, (v) => (Number.isFinite(Number(v)) && Number(v) > 0 ? clampHeight(Number(v)) : null), null))

  setOpen(open: boolean) {
    this.open = open
    write(OPEN_KEY, open ? '1' : '0')
  }

  /** Sets the height while dragging; `save` also remembers it. */
  setHeight(h: number | null, save = true) {
    this.height = h === null ? null : clampHeight(h)
    if (save) write(HEIGHT_KEY, this.height === null ? '' : String(this.height))
  }
}

export const split = new SidebarSplit()
