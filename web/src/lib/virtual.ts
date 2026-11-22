// Row positions for a list whose rows have different heights. Rows nobody has measured
// yet count as `estimate`.

/** offsets[i] is where row i starts; offsets[count] is the total height. */
export function buildOffsets(count: number, heights: (number | undefined)[], estimate: number): Float64Array {
  const out = new Float64Array(count + 1)
  for (let i = 0; i < count; i++) out[i + 1] = out[i] + (heights[i] ?? estimate)
  return out
}

/** The row that contains position y (clamped to the list). */
export function rowAt(offsets: Float64Array, y: number): number {
  const count = offsets.length - 1
  if (count <= 0) return 0
  let lo = 0
  let hi = count - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (offsets[mid] <= y) lo = mid
    else hi = mid - 1
  }
  return lo
}

/** First and last (exclusive) row to draw for a viewport, with `over` extra rows each side. */
export function visibleRange(offsets: Float64Array, top: number, height: number, over: number): [number, number] {
  const count = offsets.length - 1
  if (count <= 0) return [0, 0]
  const first = rowAt(offsets, Math.max(0, top))
  const last = rowAt(offsets, Math.max(0, top + height))
  return [Math.max(0, first - over), Math.min(count, last + 1 + over)]
}

/** "Jump to end" only helps when there is more below than fits in view (`near` px count as the end). */
export function canScroll(total: number, height: number, near = 40): boolean {
  return total - height > near
}

export interface ScrollStep {
  follow: boolean
  /** The user is acting on the list: wheel, touch, a key, or a held scrollbar. */
  user: boolean
  /** The list moved the scroll position itself since the last event. */
  programmatic: boolean
  prevTop: number
  top: number
  /** Pixels between the end of the content and the bottom of the view. */
  distance: number
  near?: number
}

/**
 * Whether the list keeps following the newest row after a scroll event. Growing content moves the
 * end away before the list pins itself again, so a distance alone is not the user scrolling up.
 */
export function followAfterScroll(s: ScrollStep): boolean {
  if (s.distance < (s.near ?? 40)) return true
  if (s.user) return false
  if (s.programmatic) return s.follow
  return s.follow && s.top >= s.prevTop
}
