import { describe, expect, it } from 'vitest'
import { checkImages, fmtBytes, MAX_IMAGE_BYTES, noImagesNote, uploadImage } from './images'

const file = (name: string, type: string, size = 100) => new File([new Uint8Array(size)], name, { type })

describe('checkImages', () => {
  it('keeps the good files and names the first problem, in the exact words', () => {
    const big = file('screenshot-4k.png', 'image/png', Math.round(9.1 * 1024 * 1024))
    const r = checkImages([file('a.png', 'image/png'), big, file('notes.pdf', 'application/pdf'), file('b.webp', 'image/webp')], 8)
    expect(r.ok.map((f) => f.name)).toEqual(['a.png', 'b.webp'])
    expect(r.error).toBe('screenshot-4k.png is 9.1 MB. Images can be up to 5 MB.')
  })

  it('refuses a non-image and says how to fix it', () => {
    expect(checkImages([file('notes.pdf', 'application/pdf')], 8).error).toBe('notes.pdf is not an image. Attach PNG, JPEG, GIF or WebP.')
  })

  it('takes only as many as fit and says so', () => {
    const r = checkImages([file('a.png', 'image/png'), file('b.png', 'image/png'), file('c.png', 'image/png')], 2)
    expect(r.ok).toHaveLength(2)
    expect(r.error).toBe('A message takes at most 8 attachments. Remove one first.')
  })

  it('accepts exactly 5 MB and all four types', () => {
    const list = ['png', 'jpeg', 'gif', 'webp'].map((t) => file('x.' + t, 'image/' + t, MAX_IMAGE_BYTES))
    expect(checkImages(list, 8)).toEqual({ ok: list, error: '' })
  })
})

describe('words', () => {
  it('formats sizes', () => {
    expect(fmtBytes(300)).toBe('1 KB')
    expect(fmtBytes(312 * 1024)).toBe('312 KB')
    expect(fmtBytes(5 * 1024 * 1024)).toBe('5.0 MB')
  })
  it('names the agent in the note', () => {
    expect(noImagesNote('Grok')).toBe('Grok does not take images. Start a chat with an agent that does, such as Claude.')
  })
})

describe('uploadImage', () => {
  const reply = (status: number, body: unknown = {}) => async () => new Response(JSON.stringify(body), { status })

  it('posts the file to the chat and returns the id', async () => {
    let seen: { url: string; init?: RequestInit } | null = null
    const f = async (url: string, init?: RequestInit) => {
      seen = { url, init }
      return new Response(JSON.stringify({ id: 'i1', name: 'a b.png', size: 100, mime: 'image/png' }), { status: 200 })
    }
    expect(await uploadImage('c 1', file('a b.png', 'image/png'), f)).toEqual({ id: 'i1', name: 'a b.png', size: 100 })
    expect(seen!.url).toBe('/api/chats/c%201/images?name=a%20b.png')
    expect(seen!.init?.method).toBe('POST')
  })

  it('turns the server refusals into plain words', async () => {
    await expect(uploadImage('c', file('a.png', 'image/png', 2048), reply(413))).rejects.toThrow('a.png is 2 KB. Images can be up to 5 MB.')
    await expect(uploadImage('c', file('a.png', 'image/png'), reply(415))).rejects.toThrow('a.png is not an image. Attach PNG, JPEG, GIF or WebP.')
    await expect(uploadImage('c', file('a.png', 'image/png'), reply(500))).rejects.toThrow('Could not attach a.png. Try again.')
  })
})
