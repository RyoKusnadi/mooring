import { describe, expect, it } from 'vitest'
import { addPage, newGraph, type GraphCommit } from './graph'

const c = (id: string, ...parents: string[]): GraphCommit => ({ id, parents })

// A merge-heavy history, newest first: two branches that merge, an octopus, a criss-cross and a root.
const history: GraphCommit[] = [
  c('m3', 'm2', 'f3'),
  c('f3', 'f2'),
  c('m2', 'm1', 'f2', 'g2'),
  c('g2', 'g1'),
  c('f2', 'f1'),
  c('x2', 'x1'),
  c('m1', 'b2', 'f1'),
  c('x1', 'b1', 'g1'),
  c('f1', 'b1'),
  c('g1', 'b1'),
  c('b2', 'b1'),
  c('b1', 'a1'),
  c('a1'),
]

function chunks<T>(list: T[], sizes: number[]): T[][] {
  const out: T[][] = []
  let at = 0
  for (const n of sizes) {
    out.push(list.slice(at, at + n))
    at += n
  }
  if (at < list.length) out.push(list.slice(at))
  return out
}

describe('lanes', () => {
  it('a straight line stays in lane 0', () => {
    const rows = addPage(newGraph(), [c('c', 'b'), c('b', 'a'), c('a')])
    expect(rows.map((r) => r.col)).toEqual([0, 0, 0])
    expect(rows[0].up).toEqual([])
    expect(rows[1].up).toEqual([{ lane: 0, color: 0 }])
    expect(rows[2].down).toEqual([])
  })

  it('a merge opens a second lane that closes where the branch starts', () => {
    const rows = addPage(newGraph(), [c('m', 'a', 'b'), c('b', 'r'), c('a', 'r'), c('r')])
    expect(rows[0].down.map((e) => e.lane)).toEqual([0, 1])
    expect(rows[0].merge).toBe(true)
    expect(rows[1].col).toBe(1)
    expect(rows[1].through.map((e) => e.lane)).toEqual([0])
    expect(rows[2].col).toBe(0)
    expect(rows[2].through.map((e) => e.lane)).toEqual([1])
    // The branch lane (1) waits for the root; the first parent's line moves over to it.
    expect(rows[2].down.map((e) => e.lane)).toEqual([1])
    expect(rows[3].up.map((e) => e.lane)).toEqual([1])
  })

  it('whole list at once equals any split into pages', () => {
    const whole = addPage(newGraph(), history)
    for (const sizes of [[1], [2, 2, 2], [3, 5], [1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1], [6], [12], [7, 1]]) {
      const g = newGraph()
      const paged = chunks(history, sizes).flatMap((p) => addPage(g, p))
      expect(paged).toEqual(whole)
    }
  })

  it('a merge whose parent is on a later page keeps its lane waiting', () => {
    const g = newGraph()
    const first = addPage(g, history.slice(0, 3))
    // m2 has parents m1, f2 and g2; f2 and g2 are on this page, m1 is not: lanes stay open at the bottom.
    expect(first[2].down.length).toBe(3)
    expect(g.lanes.filter((l) => l !== null).length).toBeGreaterThan(0)
    const rest = addPage(g, history.slice(3))
    const whole = addPage(newGraph(), history)
    expect([...first, ...rest]).toEqual(whole)
    expect(g.lanes).toEqual([])
  })

  it('a parent that is already waiting in another lane joins it', () => {
    const rows = addPage(newGraph(), [c('a', 'p'), c('b', 'p'), c('p')])
    expect(rows[1].down).toEqual([{ lane: 0, color: 0 }])
    expect(rows[1].col).toBe(1)
  })

  it('width counts the lanes drawn and never negative', () => {
    const rows = addPage(newGraph(), history)
    for (const r of rows) expect(r.width).toBeGreaterThan(r.col)
  })

  it('random merge-heavy histories give the same rows page by page', () => {
    let seed = 7
    const rand = (n: number) => ((seed = (seed * 1103515245 + 12345) & 0x7fffffff) % n)
    const ids = Array.from({ length: 400 }, (_, i) => 'c' + i)
    // Each commit points at 0 to 3 commits later in the list, so the order is topological.
    const list = ids.map((id, i) => {
      const parents = new Set<string>()
      const n = i === ids.length - 1 ? 0 : 1 + (rand(4) === 0 ? rand(3) : 0)
      for (let k = 0; k < n; k++) parents.add(ids[Math.min(ids.length - 1, i + 1 + rand(6))])
      return c(id, ...parents)
    })
    const whole = addPage(newGraph(), list)
    const g = newGraph()
    const paged: ReturnType<typeof addPage> = []
    for (let at = 0; at < list.length; ) {
      const n = 1 + rand(40)
      paged.push(...addPage(g, list.slice(at, at + n)))
      at += n
    }
    expect(paged).toEqual(whole)
    expect(Math.max(...whole.map((r) => r.width))).toBeGreaterThan(2)
  })
})
