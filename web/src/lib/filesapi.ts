// HTTP half of the files contract: list, read, save, search.
// Creating, renaming and deleting are commands on the events socket, not here.
import type { Fetcher } from './api'
import { newCommandId } from './events'

export interface Entry {
  name: string
  kind: 'file' | 'dir' | 'other' | 'link'
  size: number
  mtime: number
  hidden: boolean
  ignored: boolean
  link: boolean
  unsafe?: 'outside' | 'broken'
}
export interface Listing {
  path: string
  entries: Entry[]
  truncated: boolean
  total: number
}
export interface ReadChunk {
  path: string
  size: number
  version: string
  binary: boolean
  tooLarge: boolean
  writable: boolean
  offset: number
  next: number
  done: boolean
  content: string
}
export interface Match {
  path: string
  line: number
  col: number
  text: string
  ranges: [number, number][]
}
export interface SearchDone {
  count: number
  truncated: boolean
  engine: string
  reason: string
}
export interface SearchOptions {
  q: string
  regex?: boolean
  matchCase?: boolean
  glob?: string
  path?: string
}

export const CHUNK_BYTES = 1024 * 1024

export class FileError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly status = 0,
  ) {
    super(message)
  }
}

/** Text for a failed request that says what to do: a dropped connection reads differently from a refusal. */
export function plainError(e: unknown, fallback = 'Something went wrong. Try again.'): string {
  if (e instanceof FileError) return e.message
  if (e instanceof TypeError) return 'Could not reach the server. Check that it is running.'
  if (e instanceof Error && e.message && !/^failed to fetch|^load failed|^networkerror/i.test(e.message)) return e.message
  return /fetch|network|load failed/i.test(String((e as Error)?.message)) ? 'Could not reach the server. Check that it is running.' : fallback
}

/**
 * True when asking again can work: a dropped connection or a server fault. A refusal that names the
 * path (missing, outside the project, not a file) gives the same answer next time.
 */
export function canRetry(e: unknown): boolean {
  if (!(e instanceof FileError)) return true
  return e.status === 0 || e.status >= 500 || e.status === 408 || e.status === 429 || e.code === 'BUSY' || e.code === 'CONFLICT'
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export async function fail(res: Response): Promise<never> {
  let code = 'INTERNAL'
  let message = `The server could not do that (status ${res.status}). Try again.`
  try {
    const body = (await res.json()) as { error?: { code?: string; message?: string } }
    code = body.error?.code ?? code
    message = body.error?.message ?? message
  } catch {
    // Not JSON: keep the generic message.
  }
  throw new FileError(code, message, res.status)
}

export const qs = (o: Record<string, string | number | undefined>) =>
  Object.entries(o)
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => `${k}=${encodeURIComponent(String(v))}`)
    .join('&')

export async function getJSON<T>(url: string, f: Fetcher, signal?: AbortSignal): Promise<T> {
  const res = await f(url, { signal })
  if (!res.ok) return fail(res)
  return (await res.json()) as T
}

/** Every file path in the project, for "open file by name". */
export const listNames = (project: string, f: Fetcher = browserFetch, signal?: AbortSignal) =>
  getJSON<{ paths: string[]; truncated: boolean; engine: string }>(`/api/files/names?${qs({ project })}`, f, signal)

export const listDir = (project: string, path: string, f: Fetcher = browserFetch, signal?: AbortSignal) =>
  getJSON<Listing>(`/api/files/list?${qs({ project, path })}`, f, signal)

export const readChunk = (project: string, path: string, o: { offset?: number; limit?: number; version?: string } = {}, f: Fetcher = browserFetch) =>
  getJSON<ReadChunk>(`/api/files/read?${qs({ project, path, offset: o.offset, limit: o.limit, version: o.version })}`, f)

export interface ReadHandlers {
  onFirst: (c: ReadChunk) => void
  onMore: (text: string, loaded: number, total: number) => void
  signal?: { aborted: boolean }
}
type ChunkReader = (path: string, o: { offset?: number; limit?: number; version?: string }) => Promise<ReadChunk>

/** Reads a file in chunks. `onFirst` runs as soon as the first chunk is in; the rest follow in order. */
export async function readVia(read: ChunkReader, path: string, h: ReadHandlers): Promise<ReadChunk> {
  const first = await read(path, { limit: CHUNK_BYTES })
  h.onFirst(first)
  if (first.binary || first.tooLarge) return first
  let next = first.next
  let done = first.done
  while (!done && !h.signal?.aborted) {
    const c = await read(path, { offset: next, limit: CHUNK_BYTES, version: first.version })
    h.onMore(c.content, c.next, first.size)
    next = c.next
    done = c.done
  }
  return first
}

export const readFile = (project: string, path: string, h: ReadHandlers, f: Fetcher = browserFetch) =>
  readVia((p, o) => readChunk(project, p, o, f), path, h)

/** Saves with the version the editor read. The same id is sent again while the server answers 202. */
export async function putText(url: string, body: BodyInit, f: Fetcher, id = newCommandId(), type = 'text/plain') {
  const send = () => f(url, { method: 'PUT', headers: { 'Mooring-Command-Id': id, 'Content-Type': type }, body })
  // 202: the save is still running. Sending the same id again waits for it; a new id would save twice.
  let res = await send()
  for (let i = 0; res.status === 202 && i < 40; i++) res = await send()
  if (res.status === 202) throw new FileError('PENDING', 'The save is taking too long. Check that the server is running.')
  if (!res.ok) return fail(res)
  const out = (await res.json()) as { value: { path: string; version: string; size: number } }
  return out.value
}

export const writeFile = (project: string, path: string, base: string, text: string, f: Fetcher = browserFetch, id = newCommandId()) =>
  putText(`/api/files/content?${qs({ project, path, base })}`, text, f, id)

/** The most one upload may carry; the server counts the bytes as they arrive and refuses more. */
export const MAX_UPLOAD = 100 * 1024 * 1024

/** Address that saves a file with the browser's download. */
export const downloadUrl = (project: string, path: string) => `/api/files/download?${qs({ project, path })}`

/** Stores one file at `path`. Without `replace`, a name that is taken fails with ALREADY_EXISTS. */
export const uploadFile = (project: string, path: string, file: Blob, replace: boolean, f: Fetcher = browserFetch, id = newCommandId()) =>
  putText(`/api/files/upload?${qs({ project, path, replace: replace ? 1 : 0 })}`, file, f, id, 'application/octet-stream')

/** Where an editor reads and saves: a project, or a folder on a remote host. */
export interface FileSource {
  readChunk: ChunkReader
  readFile: (path: string, h: ReadHandlers) => Promise<ReadChunk>
  writeFile: (path: string, base: string, text: string) => Promise<{ path: string; version: string; size: number }>
}

export const localSource = (project: string): FileSource => ({
  readChunk: (p, o) => readChunk(project, p, o),
  readFile: (p, h) => readFile(project, p, h),
  writeFile: (p, b, t) => writeFile(project, p, b, t),
})

/** Parses newline-delimited JSON from a byte stream; a half line waits for the next read. */
export async function readLines(body: ReadableStream<Uint8Array>, onLine: (v: Record<string, unknown>) => void): Promise<void> {
  const reader = body.getReader()
  const dec = new TextDecoder()
  let rest = ''
  for (;;) {
    const { done, value } = await reader.read()
    rest += dec.decode(value, { stream: !done })
    const lines = rest.split('\n')
    rest = lines.pop() ?? ''
    for (const l of lines) if (l.trim()) onLine(JSON.parse(l) as Record<string, unknown>)
    if (done) break
  }
  if (rest.trim()) onLine(JSON.parse(rest) as Record<string, unknown>)
}

/** Streams matches to `onMatch`. Abort `signal` to cancel; a cancelled search resolves with null. */
export async function search(
  project: string,
  o: SearchOptions,
  onMatch: (m: Match) => void,
  signal: AbortSignal,
  f: Fetcher = browserFetch,
): Promise<SearchDone | null> {
  const url = `/api/files/search?${qs({ project, q: o.q, regex: o.regex ? 1 : undefined, case: o.matchCase ? 1 : undefined, glob: o.glob || undefined, path: o.path || undefined })}`
  let done: SearchDone | null = null
  try {
    const res = await f(url, { signal })
    if (!res.ok) return await fail(res)
    await readLines(res.body!, (l) => {
      if (l.t === 'match') onMatch(l as unknown as Match)
      else if (l.t === 'done') done = l as unknown as SearchDone
      else if (l.t === 'error') throw new FileError(String(l.code), String(l.message))
    })
  } catch (e) {
    if (signal.aborted) return null
    throw e
  }
  return done
}
