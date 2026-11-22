import { describe, expect, it } from 'vitest'
import { canRetry, FileError, listDir, listNames, readFile, readLines, search, writeFile } from './filesapi'

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status })

function streamOf(...parts: string[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder()
  return new ReadableStream({
    start(c) {
      for (const p of parts) c.enqueue(enc.encode(p))
      c.close()
    },
  })
}

describe('filesapi', () => {
  it('encodes the query and turns an error body into a FileError', async () => {
    let url = ''
    await listDir('p1', 'a b/ü', async (u) => ((url = u), json({ path: '', entries: [], truncated: false, total: 0 })))
    expect(url).toBe('/api/files/list?project=p1&path=a%20b%2F%C3%BC')
    const err = await listDir('p1', 'x', async () => json({ ok: false, error: { code: 'NOT_FOUND', message: 'No such folder.' } }, 404)).catch((e) => e)
    expect(err).toBeInstanceOf(FileError)
    expect(err.code).toBe('NOT_FOUND')
    expect(err.message).toBe('No such folder.')
  })

  it('reads the rest of a file with the first chunk\'s version', async () => {
    const urls: string[] = []
    const chunk = (offset: number, next: number, done: boolean, content: string) => ({ path: 'f', size: 6, version: 'v1', binary: false, tooLarge: false, writable: true, offset, next, done, content })
    const f = async (u: string) => {
      urls.push(u)
      return u.includes('offset=3') ? json(chunk(3, 6, true, 'def')) : json(chunk(0, 3, false, 'abc'))
    }
    const got: string[] = []
    const first = await readFile('p', 'f', { onFirst: (c) => got.push(c.content), onMore: (t) => got.push(t) }, f)
    expect(got).toEqual(['abc', 'def'])
    expect(first.version).toBe('v1')
    expect(urls[1]).toContain('version=v1')
    expect(urls[1]).toContain('offset=3')
  })

  it('stops reading when aborted', async () => {
    let calls = 0
    const f = async () => (calls++, json({ path: 'f', size: 9, version: 'v', binary: false, tooLarge: false, writable: true, offset: 0, next: 3, done: false, content: 'abc' }))
    await readFile('p', 'f', { onFirst: () => {}, onMore: () => {}, signal: { aborted: true } }, f)
    expect(calls).toBe(1)
  })

  it('saves with the base version and a command id', async () => {
    let init: RequestInit | undefined
    let url = ''
    const v = await writeFile('p', 'a.txt', 'v0', 'hi', async (u, i) => ((url = u), (init = i), json({ ok: true, value: { path: 'a.txt', version: 'v1', size: 2 } })), 'cmd1')
    expect(v.version).toBe('v1')
    expect(url).toBe('/api/files/content?project=p&path=a.txt&base=v0')
    expect((init?.headers as Record<string, string>)['Mooring-Command-Id']).toBe('cmd1')
    const err = await writeFile('p', 'a.txt', 'v0', 'hi', async () => json({ ok: false, error: { code: 'FILE_CHANGED', message: 'x' } }, 409)).catch((e) => e)
    expect(err.code).toBe('FILE_CHANGED')
  })

  it('asks for every file name of a project', async () => {
    let url = ''
    const r = await listNames('p 1', async (u) => ((url = u), json({ paths: ['a.go'], truncated: true, engine: 'rg' })))
    expect(url).toBe('/api/files/names?project=p%201')
    expect(r.truncated).toBe(true)
  })

  it('resends a pending save with the same command id', async () => {
    const ids: string[] = []
    const v = await writeFile('p', 'a.txt', 'v0', 'hi', async (_u, i) => {
      ids.push((i?.headers as Record<string, string>)['Mooring-Command-Id'])
      return ids.length < 3 ? json({ ok: false, error: { code: 'PENDING' } }, 202) : json({ ok: true, value: { path: 'a.txt', version: 'v1', size: 2 } })
    })
    expect(v.version).toBe('v1')
    expect(ids).toHaveLength(3)
    expect(new Set(ids).size).toBe(1)
  })

  it('joins NDJSON lines split across reads', async () => {
    const lines: unknown[] = []
    await readLines(streamOf('{"t":"match","seq":1}\n{"t":"ma', 'tch","seq":2}\n', '{"t":"done"}'), (l) => lines.push(l))
    expect(lines).toEqual([{ t: 'match', seq: 1 }, { t: 'match', seq: 2 }, { t: 'done' }])
  })

  it('streams matches and returns the done line', async () => {
    const f = async () =>
      new Response(streamOf('{"t":"match","seq":1,"path":"a","line":2,"col":1,"text":"x","ranges":[[0,1]]}\n{"t":"done","seq":2,"count":1,"truncated":false,"engine":"go","reason":""}\n'))
    const got: string[] = []
    const done = await search('p', { q: 'x' }, (m) => got.push(`${m.path}:${m.line}`), new AbortController().signal, f)
    expect(got).toEqual(['a:2'])
    expect(done?.count).toBe(1)
  })

  it('resolves null when cancelled and throws a search error', async () => {
    const ctl = new AbortController()
    ctl.abort()
    const cancelled = await search('p', { q: 'x' }, () => {}, ctl.signal, async () => Promise.reject(new DOMException('x', 'AbortError')))
    expect(cancelled).toBeNull()
    const err = await search('p', { q: '(' , regex: true }, () => {}, new AbortController().signal, async () => new Response(streamOf('{"t":"error","seq":1,"code":"BAD_QUERY","message":"Bad pattern."}\n'))).catch((e) => e)
    expect(err.code).toBe('BAD_QUERY')
  })
})

describe('plainError', () => {
  it('names a dropped connection and keeps server messages', async () => {
    const { plainError, FileError } = await import('./filesapi')
    expect(plainError(new TypeError('Failed to fetch'))).toBe('Could not reach the server. Check that it is running.')
    expect(plainError(new FileError('X', 'Name taken.', 409))).toBe('Name taken.')
    expect(plainError(new Error(''), 'Try again.')).toBe('Try again.')
  })
})

describe('canRetry', () => {
  it('only offers Try again where a second try can work', () => {
    expect(canRetry(new FileError('NOT_FOUND', 'x', 404))).toBe(false)
    expect(canRetry(new FileError('FORBIDDEN', 'x', 403))).toBe(false)
    expect(canRetry(new FileError('BUSY', 'x', 409))).toBe(true)
    expect(canRetry(new FileError('X', 'x', 503))).toBe(true)
    expect(canRetry(new TypeError('network'))).toBe(true)
  })
})
