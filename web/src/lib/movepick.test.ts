import { describe, expect, it } from 'vitest'
import { folderList, targets } from './movepick'

describe('folderList', () => {
  it('lists every folder above a file, once, with the project folder first', () => {
    expect(folderList(['a/b/c.txt', 'a/d.txt', 'e.txt'], ['x/y'])).toEqual(['', 'a', 'a/b', 'x', 'x/y'])
  })
})

describe('targets', () => {
  const all = ['', 'a', 'a/b', 'c']
  it('drops the current folder and the row itself with what lies inside it', () => {
    expect(targets(all, 'a/f.txt', '')).toEqual(['', 'a/b', 'c'])
    expect(targets(all, 'a', '')).toEqual(['c'])
  })
  it('narrows by what was typed, ignoring case', () => {
    expect(targets(all, 'z.txt', 'B')).toEqual(['a/b'])
  })
})
