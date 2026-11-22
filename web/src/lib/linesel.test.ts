import { describe, expect, it } from 'vitest'
import { diffRows } from './diffrows'
import type { Diff, Hunk } from './gitapi'
import { blockRows, changeRows, rangeRows, stepRow, toPairs, whyNotChoose } from './linesel'

const hunks: Hunk[] = [
  { oldStart: 1, oldLines: 4, newStart: 1, newLines: 5, header: '', lines: [[' ', 'a'], ['-', 'b'], ['+', 'B'], ['+', 'B2'], [' ', 'c']] },
  { oldStart: 20, oldLines: 2, newStart: 21, newLines: 2, header: '', lines: [['-', 'x'], ['+', 'y'], ['\\', '']] },
]
const rows = diffRows(hunks)
// rows: 0 hunk, 1 ' a', 2 -b, 3 +B, 4 +B2, 5 ' c', 6 hunk, 7 -x, 8 +y, 9 eof

describe('line choosing', () => {
  it('rows carry their hunk and line index', () => {
    expect(rows[3]).toMatchObject({ h: 0, l: 2 })
    expect(rows[8]).toMatchObject({ h: 1, l: 1 })
  })
  it('only added and removed lines can be chosen', () => {
    expect(changeRows(rows)).toEqual([2, 3, 4, 7, 8])
  })
  it('a range skips context and hunk rows, in either direction', () => {
    expect(rangeRows(rows, 2, 8)).toEqual([2, 3, 4, 7, 8])
    expect(rangeRows(rows, 4, 2)).toEqual([2, 3, 4])
  })
  it('a block is the changed lines of one hunk', () => {
    expect(blockRows(rows, 3)).toEqual([2, 3, 4])
    expect(blockRows(rows, 6)).toEqual([7, 8])
  })
  it('pairs come out in diff order', () => {
    expect(toPairs(rows, new Set([8, 3, 2]))).toEqual([[0, 1], [0, 2], [1, 1]])
    expect(toPairs(rows, [1, 5, 9])).toEqual([])
  })
  it('stepping moves to the next changeable row', () => {
    expect(stepRow(rows, 2, 1)).toBe(3)
    expect(stepRow(rows, 4, 1)).toBe(7)
    expect(stepRow(rows, 7, -1)).toBe(4)
    expect(stepRow(rows, 2, -1)).toBe(-1)
    expect(stepRow(rows, 8, 1)).toBe(-1)
  })
  it('says why a file cannot have lines chosen', () => {
    const d: Diff = { path: 'a', binary: false, tooLarge: false, hunks, version: 'v' }
    expect(whyNotChoose(d, false, false)).toBe('')
    expect(whyNotChoose({ ...d, binary: true }, false, false)).toBe('binary')
    expect(whyNotChoose({ ...d, tooLarge: true }, false, false)).toBe('large')
    expect(whyNotChoose(d, false, true)).toBe('conflict')
    expect(whyNotChoose({ ...d, origPath: 'old' }, true, false)).toBe('rename')
    expect(whyNotChoose({ ...d, origPath: 'old' }, false, false)).toBe('')
    expect(whyNotChoose({ ...d, version: undefined }, false, false)).toBe('old')
  })
})
