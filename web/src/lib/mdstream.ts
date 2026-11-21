// Incremental markdown for a reply that is still arriving. The text is cut into segments at blank
// lines outside code fences. A segment followed by another one is final: it is parsed once and
// kept. Only the last segment is parsed again when more text comes, so a long reply costs
// the size of its last block per chunk, not the size of the whole reply.
import { parseBlocks, type Block } from './md'

export interface Segment {
  /** Stable for the life of the stream, so a view can keep the node it drew. */
  id: number
  blocks: Block[]
}

interface Scan {
  pos: number
  fence: string
  seen: boolean
}

/** Scans whole lines from `st.pos`; returns the end of the first closed segment or -1. State is kept so the next call resumes. */
function scan(text: string, st: Scan): number {
  for (;;) {
    const nl = text.indexOf('\n', st.pos)
    if (nl < 0) return -1
    const line = text.slice(st.pos, nl)
    const t = line.trim()
    const f = st.fence ? null : /^ {0,3}(`{3,}|~{3,})/.exec(line)
    if (st.fence) {
      if (t.length >= st.fence.length && t === st.fence[0].repeat(t.length)) st.fence = ''
    } else if (f) {
      st.fence = f[1]
      st.seen = true
    } else if (t === '') {
      if (st.seen && nl + 1 < text.length) return nl + 1
    } else st.seen = true
    st.pos = nl + 1
  }
}

/** Where the first segment of `text` ends: the offset of the line after its blank line, or -1 while it is open. */
export function segmentEnd(text: string, start = 0): number {
  return scan(text, { pos: start, fence: '', seen: false })
}

export class MdStream {
  private src = ''
  private done: Segment[] = []
  private tailAt = 0
  private nextId = 1
  private st: Scan = { pos: 0, fence: '', seen: false }
  private cached: Segment | null = null

  /** Segments that will not change again. */
  get final(): readonly Segment[] {
    return this.done
  }

  /** The open last segment, parsed when asked for and again only after more text came. */
  get tail(): Segment {
    if (!this.cached) {
      const rest = this.src.slice(this.tailAt)
      this.cached = { id: 0, blocks: rest.trim() ? parseBlocks(rest.split('\n')) : [] }
    }
    return this.cached
  }

  /** Takes the whole text so far. */
  update(text: string): void {
    const text2 = text.includes('\r') ? text.replace(/\r\n?/g, '\n') : text
    // Text that does not extend what was seen starts over. Only the end of the kept part is compared.
    const a = Math.max(0, this.tailAt - 64)
    const bad = text2.length < this.st.pos || text2.slice(a, this.tailAt) !== this.src.slice(a, this.tailAt) || text2.slice(this.tailAt, this.st.pos) !== this.src.slice(this.tailAt, this.st.pos)
    if (bad) {
      this.done = []
      this.tailAt = 0
      this.st = { pos: 0, fence: '', seen: false }
    }
    this.src = text2
    this.cached = null
    for (let end = scan(text2, this.st); end >= 0; end = scan(text2, this.st)) {
      const seg = text2.slice(this.tailAt, end)
      if (seg.trim()) this.done.push({ id: this.nextId++, blocks: parseBlocks(seg.split('\n')) })
      this.tailAt = end
      this.st = { pos: end, fence: '', seen: false }
    }
  }
}
