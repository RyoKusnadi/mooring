import { expect, it } from 'vitest'
import { chunks } from './chunks'

it('splits 2500 paths into 1000, 1000 and 500 in order', () => {
  const all = Array.from({ length: 2500 }, (_, i) => String(i))
  const parts = chunks(all, 1000)
  expect(parts.map((p) => p.length)).toEqual([1000, 1000, 500])
  expect(parts.flat()).toEqual(all)
})
it('keeps one empty piece for an empty list and one piece for exactly the size', () => {
  expect(chunks([], 1000)).toEqual([[]])
  expect(chunks(Array(1000).fill('a'), 1000)).toHaveLength(1)
})
