import { describe, expect, it } from 'vitest'
import { applyFiles, type EditTarget, type LiveDoc } from './multiedit'

const edit = (l: number, c1: number, c2: number, newText: string) => ({ range: { start: { line: l, character: c1 }, end: { line: l, character: c2 } }, newText })

function target(disk: Record<string, string>, open: Record<string, string> = {}, held: string[] = []) {
  const saved: Record<string, { base: string; text: string }> = {}
  const t: EditTarget = {
    live: (p): LiveDoc | null =>
      p in open
        ? {
            text: () => open[p],
            change: (cs) => {
              for (const c of cs) open[p] = open[p].slice(0, c.from) + c.insert + open[p].slice(c.to)
            },
          }
        : null,
    heldUnsaved: (p) => held.includes(p),
    read: async (p) => (p in disk ? { text: disk[p], version: 'v-' + p } : null),
    write: async (p, base, text) => {
      if (p === 'locked.go') throw new Error('The file changed on disk.')
      saved[p] = { base, text }
    },
  }
  return { t, saved, open }
}

describe('applyFiles', () => {
  it('changes an open editor in place and saves other files against the version read', async () => {
    const { t, saved, open } = target({ 'b.go': 'foo()\n' }, { 'a.go': 'x := foo\n' })
    const r = await applyFiles([{ path: 'a.go', edits: [edit(0, 5, 8, 'bar')] }, { path: 'b.go', edits: [edit(0, 0, 3, 'bar')] }], t)
    expect(r).toEqual({ changed: ['a.go', 'b.go'], failed: [] })
    expect(open['a.go']).toBe('x := bar\n')
    expect(saved['b.go']).toEqual({ base: 'v-b.go', text: 'bar()\n' })
  })

  it('reports a file it could not change and goes on with the rest', async () => {
    const { t } = target({ 'locked.go': 'foo', 'c.go': 'foo' })
    const r = await applyFiles([{ path: 'locked.go', edits: [edit(0, 0, 3, 'bar')] }, { path: 'gone.go', edits: [edit(0, 0, 3, 'bar')] }, { path: 'c.go', edits: [edit(0, 0, 3, 'bar')] }], t)
    expect(r).toEqual({ changed: ['c.go'], failed: [{ path: 'locked.go', reason: 'The file changed on disk.' }, { path: 'gone.go', reason: 'The file is binary or too large to change here.' }] })
  })

  it('leaves a file whose text changed after the plan was made', async () => {
    const { t, saved, open } = target({ 'b.go': 'foo()\n' }, { 'a.go': 'x := foo\n' })
    const r = await applyFiles([{ path: 'a.go', edits: [edit(0, 5, 8, 'bar')], planned: 'x := fo\n' }, { path: 'b.go', edits: [edit(0, 0, 3, 'bar')], planned: 'foo()\n' }], t)
    expect(r).toEqual({ changed: ['b.go'], failed: [{ path: 'a.go', reason: 'The file changed after the preview, so it was left alone. Preview again.' }] })
    expect(open['a.go']).toBe('x := foo\n')
    expect(saved['b.go'].text).toBe('bar()\n')
  })

  it('refuses before changing anything when a hidden tab holds unsaved changes', async () => {
    const { t, saved } = target({ 'a.go': 'foo', 'b.go': 'foo' }, {}, ['b.go'])
    const r = await applyFiles([{ path: 'a.go', edits: [edit(0, 0, 3, 'bar')] }, { path: 'b.go', edits: [edit(0, 0, 3, 'bar')] }], t)
    expect(r).toHaveProperty('refused')
    expect(saved).toEqual({})
  })
})
