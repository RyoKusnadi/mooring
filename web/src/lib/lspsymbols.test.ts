import { describe, expect, it } from 'vitest'
import { parseSymbols, symbolPath } from './lspsymbols'

const r = (l1: number, c1: number, l2: number, c2: number) => ({ start: { line: l1, character: c1 }, end: { line: l2, character: c2 } })

describe('symbol path at the cursor', () => {
  const tree = parseSymbols([
    { name: 'Server', kind: 5, range: r(2, 0, 9, 1), selectionRange: r(2, 5, 2, 11), children: [{ name: 'handleLogin', kind: 6, range: r(4, 1, 7, 2), selectionRange: r(4, 6, 4, 17) }] },
    { name: 'other', kind: 12, range: r(11, 0, 11, 14), selectionRange: r(11, 5, 11, 10) },
  ])

  it('lists outer then inner symbols with the place of each name', () => {
    expect(symbolPath(tree, { line: 5, character: 3 })).toEqual([
      { name: 'Server', line: 2, character: 5 },
      { name: 'handleLogin', line: 4, character: 6 },
    ])
  })

  it('stops at the outer symbol when the cursor is outside its children', () => {
    expect(symbolPath(tree, { line: 3, character: 0 }).map((s) => s.name)).toEqual(['Server'])
  })

  it('is empty between symbols', () => {
    expect(symbolPath(tree, { line: 10, character: 0 })).toEqual([])
  })

  it('takes the end of a range as inside', () => {
    expect(symbolPath(tree, { line: 11, character: 14 }).map((s) => s.name)).toEqual(['other'])
  })
})

describe('parseSymbols', () => {
  it('nests a flat list by range', () => {
    const tree = parseSymbols([
      { name: 'run', location: { uri: 'file:///a', range: r(5, 0, 6, 1) } },
      { name: 'App', location: { uri: 'file:///a', range: r(1, 0, 9, 1) } },
    ])
    expect(tree).toHaveLength(1)
    expect(symbolPath(tree, { line: 5, character: 2 }).map((s) => s.name)).toEqual(['App', 'run'])
  })

  it('drops malformed entries and non-lists', () => {
    expect(parseSymbols(null)).toEqual([])
    expect(parseSymbols({})).toEqual([])
    expect(parseSymbols([{ name: 3, range: r(0, 0, 0, 1) }, { name: 'x' }, { name: 'y', range: { start: { line: -1, character: 0 }, end: { line: 0, character: 0 } } }])).toEqual([])
  })

  it('falls back to the range start when there is no selection range', () => {
    expect(symbolPath(parseSymbols([{ name: 'z', range: r(3, 2, 4, 0) }]), { line: 3, character: 5 })).toEqual([{ name: 'z', line: 3, character: 2 }])
  })
})
