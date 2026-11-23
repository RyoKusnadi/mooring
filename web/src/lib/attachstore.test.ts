import { beforeEach, describe, expect, it } from 'vitest'
import { attach, imageCount, MAX_ATTACHMENTS, wire } from './attachstore.svelte'

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

  it('keeps images by id, once, and counts them in the eight', () => {
    const img = (n: number) => ({ kind: 'image' as const, imageId: 'i' + n, name: `s${n}.png`, size: 1000 })
    expect(attach.add('c', img(1))).toBe('')
    expect(attach.add('c', img(1))).toBe('')
    expect(attach.list('c')).toHaveLength(1)
    for (let i = 2; i <= MAX_ATTACHMENTS - 1; i++) attach.add('c', img(i))
    attach.add('c', sel(1))
    expect(attach.list('c')).toHaveLength(MAX_ATTACHMENTS)
    expect(attach.add('c', img(99))).toBe('A message takes at most 8 attachments. Remove one first.')
    expect(imageCount(attach.list('c'))).toBe(MAX_ATTACHMENTS - 1)
  })

  it('sends an image as an id and a name, never its bytes or size', () => {
    expect(wire([{ kind: 'image', imageId: 'i1', name: 'a.png', size: 5 }])).toEqual([{ kind: 'image', imageId: 'i1', name: 'a.png' }])
  })

  it('collects terminal text for the composer to take once', () => {
    attach.addText('c', 'one')
    attach.addText('c', 'two')
    expect(attach.takeText('c')).toBe('one\n\ntwo')
    expect(attach.takeText('c')).toBe('')
  })
})
