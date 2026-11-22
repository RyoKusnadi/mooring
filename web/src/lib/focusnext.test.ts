import { expect, it } from 'vitest'
import { survivor } from './focusnext'

const rows = ['a', 'b', 'c', 'd']

it('keeps the row when it is still there', () => {
  expect(survivor(rows, ['a', 'b'], 'b')).toBe('b')
})
it('moves to the next row that survives', () => {
  expect(survivor(rows, ['a', 'd'], 'b')).toBe('d')
})
it('falls back to the previous row when nothing after survives', () => {
  expect(survivor(rows, ['a', 'b'], 'c')).toBe('b')
})
it('gives nothing when every row is gone', () => {
  expect(survivor(rows, [], 'b')).toBe('')
})
it('starts at the first row for an unknown one', () => {
  expect(survivor(rows, ['c'], 'zzz')).toBe('c')
})
