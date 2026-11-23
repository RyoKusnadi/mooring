import { beforeEach, describe, expect, it } from 'vitest'
import { attach } from './attachstore.svelte'
import { ImagePicker } from './imagepick.svelte'

const png = (name: string, size = 100) => new File([new Uint8Array(size)], name, { type: 'image/png' })

beforeEach(() => {
  attach.pending = {}
  globalThis.fetch = (async (_url: string, init: RequestInit) => {
    const name = ((init.body as File).name ?? 'x').replace(/\.png$/, '')
    return new Response(JSON.stringify({ id: 'i-' + name, name: name + '.png', size: (init.body as File).size, mime: 'image/png' }), { status: 200 })
  }) as typeof fetch
})

describe('ImagePicker', () => {
  it('uploads, then attaches in the order the files were given', async () => {
    let focused = 0
    const p = new ImagePicker({ id: () => 'c', takes: () => true, done: () => focused++ })
    await p.add([png('a.png'), png('b.png')])
    expect(attach.list('c').map((a) => a.kind === 'image' && a.imageId)).toEqual(['i-a', 'i-b'])
    expect(p.error).toBe('')
    expect(p.busy).toBe(0)
    expect(focused).toBe(1)
  })

  it('keeps the good files when one is refused', async () => {
    const p = new ImagePicker({ id: () => 'c', takes: () => true, done: () => {} })
    await p.add([png('a.png'), new File(['x'], 'notes.txt', { type: 'text/plain' })])
    expect(attach.list('c')).toHaveLength(1)
    expect(p.error).toBe('notes.txt is not an image. Attach PNG, JPEG, GIF or WebP.')
  })

  it('uploads nothing when the agent takes no images', async () => {
    let calls = 0
    globalThis.fetch = (async () => (calls++, new Response('{}'))) as typeof fetch
    const p = new ImagePicker({ id: () => 'c', takes: () => false, done: () => {} })
    await p.add([png('a.png')])
    expect(calls).toBe(0)
    expect(p.refused).toBe(true)
    expect(attach.list('c')).toEqual([])
  })

  it('counts what is attached already against the eight', async () => {
    const p = new ImagePicker({ id: () => 'c', takes: () => true, done: () => {} })
    for (let i = 0; i < 7; i++) attach.add('c', { kind: 'image', imageId: 'o' + i, name: 'o.png', size: 1 })
    await p.add([png('a.png'), png('b.png')])
    expect(attach.list('c')).toHaveLength(8)
    expect(p.error).toBe('A message takes at most 8 attachments. Remove one first.')
  })
})
