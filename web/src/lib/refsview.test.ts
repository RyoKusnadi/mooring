import { describe, expect, it } from 'vitest'
import { groupByPath, markName, nameProblem, refsSummary, renameSummary } from './refsview'

const edit = { range: { start: { line: 0, character: 0 }, end: { line: 0, character: 1 } }, newText: 'x' }

describe('refsview', () => {
  it('groups neighbours by path', () => {
    const g = groupByPath([{ path: 'a.go' }, { path: 'a.go' }, { path: 'b.go' }])
    expect(g.map((x) => [x.path, x.items.length])).toEqual([['a.go', 2], ['b.go', 1]])
  })
  it('marks every occurrence of the name', () => {
    expect(markName('f(f)', 'f').filter((s) => s.hit).length).toBe(2)
    expect(markName('abc', '')).toEqual([{ text: 'abc', hit: false }])
    expect(markName('', 'x')).toEqual([])
  })
  it('counts', () => {
    expect(refsSummary(1, 1)).toBe('1 reference in 1 file')
    expect(renameSummary([{ path: 'a', edits: [edit, edit] }, { path: 'b', edits: [edit] }])).toBe('Rename in 2 files, 3 places')
  })
  it('refuses an empty or unchanged name', () => {
    expect(nameProblem('a', '  ')).not.toBe('')
    expect(nameProblem('a', 'a')).not.toBe('')
    expect(nameProblem('a', 'b')).toBe('')
  })
})

describe('replaceSummary', () => {
  it('counts files and places', async () => {
    const { replaceSummary } = await import('./refsview')
    expect(replaceSummary(1, 1)).toBe('Replace in 1 file, 1 place')
    expect(replaceSummary(2, 5)).toBe('Replace in 2 files, 5 places')
  })
})
