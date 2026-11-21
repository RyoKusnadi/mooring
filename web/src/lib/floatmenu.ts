// A menu that a scrolling list cannot clip. The menu element goes into the browser's top layer
// (popover) and is placed with fixed coordinates next to its button: under it, or above it when
// there is no room below, and always inside the window. It closes when anything scrolls or the
// window resizes. Escape and outside clicks stay with the component that owns the menu.

export interface FloatOptions {
  /** The button the menu belongs to. */
  anchor: () => HTMLElement | undefined
  close: () => void
}

const GAP = 2
const EDGE = 4

/** Where a menu of this size goes: the top-left corner, right-aligned to the anchor, in the window. */
export function place(anchor: DOMRect, size: { width: number; height: number }, win: { width: number; height: number }) {
  const left = Math.max(EDGE, Math.min(anchor.right - size.width, win.width - size.width - EDGE))
  const below = anchor.bottom + GAP
  const above = anchor.top - GAP - size.height
  const fitsBelow = below + size.height <= win.height - EDGE
  const top = fitsBelow || above < EDGE ? Math.max(EDGE, Math.min(below, win.height - size.height - EDGE)) : above
  return { left, top }
}

export function floatMenu(node: HTMLElement, opts: FloatOptions) {
  let current = opts
  node.popover = 'manual'
  node.style.position = 'fixed'
  node.style.inset = 'auto'
  node.style.margin = '0'
  node.style.maxHeight = `${window.innerHeight - 2 * EDGE}px`
  node.style.overflowY = 'auto'
  node.showPopover()

  const move = () => {
    const a = current.anchor()
    if (!a) return
    const { left, top } = place(a.getBoundingClientRect(), { width: node.offsetWidth, height: node.offsetHeight }, { width: window.innerWidth, height: window.innerHeight })
    node.style.left = `${left}px`
    node.style.top = `${top}px`
  }
  const onScroll = (e: Event) => {
    const t = e.target
    if (t instanceof Node && node.contains(t)) return
    // Only scrolling that moves the button matters: the page itself, or a box that holds it.
    const a = current.anchor()
    if (t instanceof Element && a && !t.contains(a)) return
    current.close()
  }
  move()
  // The menu changes size when it asks "Remove from list?".
  const watch = new ResizeObserver(move)
  watch.observe(node)
  window.addEventListener('scroll', onScroll, true)
  const onResize = () => current.close()
  window.addEventListener('resize', onResize)
  return {
    update(next: FloatOptions) {
      current = next
    },
    destroy() {
      watch.disconnect()
      window.removeEventListener('scroll', onScroll, true)
      window.removeEventListener('resize', onResize)
      try {
        node.hidePopover()
      } catch {
        // Already gone with its parent.
      }
    },
  }
}
