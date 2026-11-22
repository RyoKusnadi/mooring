import { describe, expect, it } from 'vitest'
import { diffCounts, diffRows } from './diffrows'
import type { Hunk } from './gitapi'

const hunk: Hunk = {
  oldStart: 10,
  oldLines: 3,
  newStart: 10,
  newLines: 3,
  header: 'func main',
  lines: [
    [' ', 'a'],
    ['-', 'b'],
    ['+', 'B'],
    ['+', 'C'],
    ['\\', 'No newline at end of file'],
  ],
}

describe('diffRows', () => {
  it('numbers old and new lines on their own sides', () => {
    const rows = diffRows([hunk])
    expect(rows[0]).toEqual({ type: 'hunk', text: 'Lines 10 to 12 changed, near func main', h: 0 })
    expect(rows.slice(1).map((r) => (r.type === 'line' ? [r.kind, r.old, r.now] : []))).toEqual([
      [' ', 10, 10],
      ['-', 11, null],
      ['+', null, 11],
      ['+', null, 12],
      ['\\', null, null],
    ])
  })

  it('describes an empty side of a new or deleted file', () => {
    const added: Hunk = { oldStart: 0, oldLines: 0, newStart: 1, newLines: 1, header: '', lines: [['+', 'x']] }
    expect(diffRows([added])[0]).toEqual({ type: 'hunk', text: 'Added line 1', h: 0 })
    const gone: Hunk = { oldStart: 1, oldLines: 2, newStart: 0, newLines: 0, header: '', lines: [['-', 'a'], ['-', 'b']] }
    expect(diffRows([gone])[0]).toEqual({ type: 'hunk', text: 'Removed lines 1 to 2', h: 0 })
  })

  it('counts added and removed lines', () => {
    expect(diffCounts([hunk])).toEqual({ added: 2, removed: 1 })
  })
})
