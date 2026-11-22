import { describe, expect, it } from 'vitest'
import { applyEdits, countEdits, offsetOf, planEdit } from './lspedits'

const at = (line: number, character: number) => ({ line, character })
const edit = (l: number, c1: number, c2: number, newText: string) => ({ range: { start: at(l, c1), end: at(l, c2) }, newText })
const pathOf = (uri: string) => (uri.startsWith('file:///p/') ? uri.slice('file:///p/'.length) : null)

describe('offsetOf', () => {
  it('counts lines and stops at the end of a line, before a CR', () => {
    const t = 'ab\r\ncd\nef'
    expect(offsetOf(t, at(0, 1))).toBe(1)
    expect(offsetOf(t, at(0, 9))).toBe(2)
    expect(offsetOf(t, at(1, 1))).toBe(5)
    expect(offsetOf(t, at(2, 2))).toBe(9)
    expect(offsetOf(t, at(7, 0))).toBe(9)
  })
})

describe('applyEdits', () => {
  it('makes every edit, in any order given, and refuses overlapping ones', () => {
    const t = 'let foo = 1\nfoo + foo\n'
    expect(applyEdits(t, [edit(1, 0, 3, 'bar'), edit(0, 4, 7, 'bar'), edit(1, 6, 9, 'bar')])).toBe('let bar = 1\nbar + bar\n')
    expect(applyEdits(t, [edit(0, 0, 5, 'x'), edit(0, 3, 7, 'y')])).toBeNull()
  })
})

describe('planEdit', () => {
  it('reads both shapes of WorkspaceEdit, sorted by path, and counts the places', () => {
    const changes = { 'file:///p/b.go': [edit(0, 0, 1, 'x')], 'file:///p/a.go': [edit(0, 0, 1, 'x'), edit(2, 0, 1, 'x')] }
    const plan = planEdit({ changes }, pathOf)
    expect(plan.ok && plan.files.map((f) => f.path)).toEqual(['a.go', 'b.go'])
    expect(plan.ok && countEdits(plan.files)).toBe(3)
    const doc = planEdit({ documentChanges: [{ textDocument: { uri: 'file:///p/a.go', version: 1 }, edits: [edit(0, 0, 1, 'x')] }] }, pathOf)
    expect(doc.ok && doc.files[0].path).toBe('a.go')
  })

  it('refuses file operations and files outside the project', () => {
    expect(planEdit({ documentChanges: [{ kind: 'rename', oldUri: 'file:///p/a', newUri: 'file:///p/b' }] }, pathOf).ok).toBe(false)
    expect(planEdit({ changes: { 'file:///elsewhere/a.go': [edit(0, 0, 1, 'x')] } }, pathOf).ok).toBe(false)
    expect(planEdit(null, pathOf)).toEqual({ ok: true, files: [] })
  })
})
