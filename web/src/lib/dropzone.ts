// Where a dragged tab would land. Pure so the thresholds can be tested.
import type { DropTarget } from './layout-edit'

export const EDGE = 0.25

export function zoneAt(rect: { left: number; top: number; width: number; height: number }, x: number, y: number): DropTarget {
  const fx = (x - rect.left) / rect.width
  const fy = (y - rect.top) / rect.height
  if (fx < EDGE) return 'left'
  if (fx > 1 - EDGE) return 'right'
  if (fy < EDGE) return 'top'
  if (fy > 1 - EDGE) return 'bottom'
  return 'center'
}

/** Index a tab dropped at pointer x would take among the tab rectangles. */
export function insertIndex(tabs: { left: number; width: number }[], x: number): number {
  const i = tabs.findIndex((r) => x < r.left + r.width / 2)
  return i < 0 ? tabs.length : i
}
