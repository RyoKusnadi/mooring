// The lines chosen in one diff, and the ways to choose them: click, shift-click, drag, a whole block, keys.
// Rows are indexes into diffRows(); only changed lines count (see linesel.ts).
import type { DiffRow } from './diffrows'
import { blockRows, rangeRows } from './linesel'

export class LinePick {
  /** Chosen row indexes. Replaced, never changed in place, so views see every change. */
  chosen = $state.raw<ReadonlySet<number>>(new Set())
  /** The row a shift-click or a drag starts from. */
  anchor = $state(-1)
  private base: ReadonlySet<number> = new Set()
  private drag: 'add' | 'remove' | null = null

  get size() {
    return this.chosen.size
  }

  has(i: number) {
    return this.chosen.has(i)
  }

  clear() {
    this.chosen = new Set()
    this.anchor = -1
    this.drag = null
  }

  toggle(i: number) {
    const next = new Set(this.chosen)
    if (!next.delete(i)) next.add(i)
    this.chosen = next
    this.anchor = i
  }

  /** Adds the changed lines between the anchor and row `i`. */
  extend(rows: DiffRow[], i: number) {
    const from = this.anchor < 0 ? i : this.anchor
    this.chosen = new Set([...this.chosen, ...rangeRows(rows, from, i)])
  }

  /** The whole block that row `i` is in: chosen, or all unchosen when it already was whole. */
  block(rows: DiffRow[], i: number) {
    const all = blockRows(rows, i)
    const whole = all.length > 0 && all.every((k) => this.chosen.has(k))
    const next = new Set(this.chosen)
    for (const k of all) whole ? next.delete(k) : next.add(k)
    this.chosen = next
    this.anchor = i
  }

  /** Whether every changed line of the block holding row `i` is chosen. */
  wholeBlock(rows: DiffRow[], i: number) {
    const all = blockRows(rows, i)
    return all.length > 0 && all.every((k) => this.chosen.has(k))
  }

  dragStart(i: number) {
    this.base = this.chosen
    this.drag = this.chosen.has(i) ? 'remove' : 'add'
    this.anchor = i
    this.dragOver(null, i)
  }

  dragOver(rows: DiffRow[] | null, i: number) {
    if (!this.drag) return
    const span = rows ? rangeRows(rows, this.anchor, i) : [i]
    const next = new Set(this.base)
    for (const k of span) this.drag === 'add' ? next.add(k) : next.delete(k)
    this.chosen = next
  }

  get dragging() {
    return this.drag !== null
  }

  dragEnd() {
    this.drag = null
  }
}
