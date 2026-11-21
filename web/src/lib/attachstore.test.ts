import { beforeEach, describe, expect, it } from 'vitest'
import { attach, MAX_ATTACHMENTS, wire } from './attachstore.svelte'

const sel = (from: number, text = 'x') => ({ kind: 'selection' as const, path: 'a.go', from, to: from + 1, text })

beforeEach(() => {
  attach.pending = {}
  attach.texts = {}
})

describe('attachments', () => {
  it('adds, replaces the same lines, and removes', () => {
    expect(attach.add('c', sel(1))).toBe('')
    expect(attach.add('c', sel(1, 'y'))).toBe('')
    expect(attach.add('c', sel(5))).toBe('')
    expect(attach.list('c')).toHaveLength(2)
    attach.remove('c', 0)
    expect(attach.list('c').map((a) => a.kind === 'selection' && a.from)).toEqual([5])
    attach.clear('c')
    expect(attach.list('c')).toEqual([])
  })

  it('refuses a ninth attachment and says why', () => {
    for (let i = 0; i < MAX_ATTACHMENTS; i++) attach.add('c', sel(i * 3 + 1))
    expect(attach.add('c', sel(99))).toMatch(/at most 8/)
  })

  it('refuses more than 64 KB of text', () => {
    expect(attach.add('c', sel(1, 'a'.repeat(40000)))).toBe('')
    expect(attach.add('c', sel(5, 'a'.repeat(40000)))).toMatch(/64 KB/)
  })

  it('sends an output by id and a selection with its text', () => {
    expect(wire([{ kind: 'output', outputId: 'o1', lines: 9 }, sel(1)])).toEqual([{ kind: 'output', outputId: 'o1' }, sel(1)])
  })

  it('collects terminal text for the composer to take once', () => {
    attach.addText('c', 'one')
    attach.addText('c', 'two')
    expect(attach.takeText('c')).toBe('one\n\ntwo')
    expect(attach.takeText('c')).toBe('')
  })
})
