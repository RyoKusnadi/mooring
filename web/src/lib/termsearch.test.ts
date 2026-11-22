import { describe, expect, it } from 'vitest'
import { findAll, nextIndex } from './termsearch'

describe('findAll', () => {
  it('finds every hit top to bottom, ignoring case for a lower-case query', () => {
    expect(findAll(['Error one', 'ok', 'error error'], 'error')).toEqual([
      { row: 0, col: 0, len: 5 },
      { row: 2, col: 0, len: 5 },
      { row: 2, col: 6, len: 5 },
    ])
  })

  it('matches case exactly once the query has a capital, and finds nothing for an empty query', () => {
    expect(findAll(['Error', 'error'], 'Error')).toEqual([{ row: 0, col: 0, len: 5 }])
    expect(findAll(['x'], '')).toEqual([])
  })
})

describe('nextIndex', () => {
  it('starts at the newest hit, goes up to older and down to newer, and wraps', () => {
    expect(nextIndex(3, -1, 'up')).toBe(2)
    expect(nextIndex(3, 2, 'up')).toBe(1)
    expect(nextIndex(3, 0, 'up')).toBe(2)
    expect(nextIndex(3, 2, 'down')).toBe(0)
    expect(nextIndex(0, 1, 'down')).toBe(-1)
  })
})
