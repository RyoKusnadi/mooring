import { describe, expect, it } from 'vitest'
import { diffRows } from './diffrows'
import type { Hunk } from './gitapi'
import { LinePick } from './linepick.svelte'

const hunks: Hunk[] = [
  { oldStart: 1, oldLines: 3, newStart: 1, newLines: 4, header: '', lines: [[' ', 'a'], ['-', 'b'], ['+', 'B'], ['+', 'C'], [' ', 'c']] },
  { oldStart: 20, oldLines: 1, newStart: 21, newLines: 1, header: '', lines: [['-', 'x'], ['+', 'y']] },
]
// rows: 0 hunk, 1 a, 2 -b, 3 +B, 4 +C, 5 c, 6 hunk, 7 -x, 8 +y
const rows = diffRows(hunks)
const list = (p: LinePick) => [...p.chosen].sort((a, b) => a - b)

describe('LinePick', () => {
  it('a click toggles one line and sets the anchor', () => {
    const p = new LinePick()
    p.toggle(3)
    expect(list(p)).toEqual([3])
    p.toggle(3)
    expect(list(p)).toEqual([])
    expect(p.anchor).toBe(3)
  })
  it('shift-click chooses every changed line from the anchor, across hunks', () => {
    const p = new LinePick()
    p.toggle(2)
    p.extend(rows, 7)
    expect(list(p)).toEqual([2, 3, 4, 7])
  })
  it('a block is chosen whole, and chosen again it is cleared', () => {
    const p = new LinePick()
    p.block(rows, 4)
    expect(list(p)).toEqual([2, 3, 4])
    expect(p.wholeBlock(rows, 3)).toBe(true)
    p.block(rows, 3)
    expect(list(p)).toEqual([])
  })
  it('a block with some lines chosen is completed', () => {
    const p = new LinePick()
    p.toggle(3)
    p.block(rows, 3)
    expect(list(p)).toEqual([2, 3, 4])
  })
  it('a drag over unchosen lines chooses them and moving back gives them up', () => {
    const p = new LinePick()
    p.dragStart(2)
    p.dragOver(rows, 4)
    expect(list(p)).toEqual([2, 3, 4])
    p.dragOver(rows, 3)
    expect(list(p)).toEqual([2, 3])
    p.dragEnd()
    p.dragOver(rows, 8)
    expect(list(p)).toEqual([2, 3])
  })
  it('a drag that starts on a chosen line removes lines', () => {
    const p = new LinePick()
    p.block(rows, 2)
    p.dragStart(3)
    p.dragOver(rows, 4)
    expect(list(p)).toEqual([2])
  })
  it('clear forgets the choice', () => {
    const p = new LinePick()
    p.block(rows, 2)
    p.clear()
    expect(p.size).toBe(0)
    expect(p.anchor).toBe(-1)
  })
})
