import { describe, expect, it } from 'vitest'
import { countProblems, countText, limitProblems, severityName, sortProblems, toProblems, type Problem } from './problems'
import { problems } from './problems.svelte'

const diag = (line: number, severity: number, message: string) => ({ range: { start: { line, character: 2 } }, severity, message, source: 'gopls' })

describe('toProblems', () => {
  it('counts lines and columns from 1 and treats a missing severity as an error', () => {
    expect(toProblems('a.go', [diag(0, 2, ' unused x ')])).toEqual([{ path: 'a.go', line: 1, col: 3, severity: 2, message: 'unused x', source: 'gopls' }])
    expect(toProblems('a.go', [{ message: 'bad' }])[0]).toMatchObject({ line: 1, col: 1, severity: 1 })
    expect(toProblems('a.go', null)).toEqual([])
  })
})

describe('sortProblems and countProblems', () => {
  it('puts errors first, then by path and line, and counts errors and warnings', () => {
    const list = [...toProblems('b.go', [diag(4, 2, 'w')]), ...toProblems('b.go', [diag(9, 1, 'e2')]), ...toProblems('a.go', [diag(3, 1, 'e1'), diag(1, 3, 'i')])]
    expect(sortProblems(list).map((p) => p.message)).toEqual(['e1', 'e2', 'w', 'i'])
    expect(countProblems(list)).toEqual({ errors: 2, warnings: 1 })
  })
})

describe('problems store', () => {
  it('keeps each file per server and project, clears a file on an empty list, and drops a server', () => {
    problems.set('k1', 'p1', 'a.go', toProblems('a.go', [diag(0, 1, 'e')]))
    problems.set('k2', 'p1', 'b.ts', toProblems('b.ts', [diag(0, 2, 'w')]))
    problems.set('k3', 'p2', 'c.go', toProblems('c.go', [diag(0, 1, 'other project')]))
    expect(problems.counts('p1')).toEqual({ errors: 1, warnings: 1 })
    problems.set('k1', 'p1', 'a.go', [])
    expect(problems.list('p1').map((p) => p.path)).toEqual(['b.ts'])
    problems.drop('k2')
    expect(problems.list('p1')).toEqual([])
    expect(problems.counts('p2').errors).toBe(1)
  })
})

describe('problem text', () => {
  const p = (n: number): Problem[] => Array.from({ length: n }, (_, i) => ({ path: 'a.go', line: i + 1, col: 1, severity: 1, message: 'm', source: '' }))
  it('names severities and falls back to error', () => {
    expect([1, 2, 3, 4, 9].map(severityName)).toEqual(['Error', 'Warning', 'Information', 'Hint', 'Error'])
  })
  it('counts in words, with singular and nothing', () => {
    expect(countText({ errors: 2, warnings: 5 })).toBe('2 errors, 5 warnings')
    expect(countText({ errors: 1, warnings: 0 })).toBe('1 error')
    expect(countText({ errors: 0, warnings: 1 })).toBe('1 warning')
    expect(countText({ errors: 0, warnings: 0 })).toBe('')
  })
  it('limits the list and says how many were left out', () => {
    expect(limitProblems(p(5), 3)).toMatchObject({ more: 2 })
    expect(limitProblems(p(5), 3).shown).toHaveLength(3)
    expect(limitProblems(p(2), 3)).toMatchObject({ more: 0 })
  })
})
