// Search in a terminal's scrollback. findAll is plain; TermFind walks the hits of one xterm.
import type { Terminal } from '@xterm/xterm'

export interface Hit {
  row: number
  col: number
  len: number
}

/** Where the search is: hit `at` (1 is the oldest) of `total`; at is 0 when nothing matches. */
export interface FindState {
  at: number
  total: number
}

const MAX_HITS = 1000

/** Hits of query in lines, top to bottom. Lower-case queries ignore case; one capital makes it exact. */
export function findAll(lines: string[], query: string): Hit[] {
  if (!query) return []
  const exact = query !== query.toLowerCase()
  const q = exact ? query : query.toLowerCase()
  const hits: Hit[] = []
  for (let row = 0; row < lines.length && hits.length < MAX_HITS; row++) {
    const line = exact ? lines[row] : lines[row].toLowerCase()
    for (let col = line.indexOf(q); col >= 0 && hits.length < MAX_HITS; col = line.indexOf(q, col + q.length)) {
      hits.push({ row, col, len: q.length })
    }
  }
  return hits
}

/** The next hit: up goes to older output, down to newer, and both wrap. -1 is "none yet": start at the newest. */
export function nextIndex(total: number, current: number, dir: 'up' | 'down'): number {
  if (total === 0) return -1
  if (current < 0 || current >= total) return total - 1
  return dir === 'up' ? (current - 1 + total) % total : (current + 1) % total
}

export class TermFind {
  private query = ''
  private index = -1

  constructor(private term: Terminal) {}

  /** dir 'same' finds the newest hit of a changed query, or keeps the place for the same query. */
  find(query: string, dir: 'up' | 'down' | 'same'): FindState {
    const buf = this.term.buffer.active
    const lines: string[] = []
    for (let i = 0; i < buf.length; i++) lines.push(buf.getLine(i)?.translateToString(true) ?? '')
    const hits = findAll(lines, query)
    if (query !== this.query) {
      this.query = query
      this.index = -1
    }
    this.index = dir === 'same' && this.index >= 0 && this.index < hits.length ? this.index : nextIndex(hits.length, this.index, dir === 'same' ? 'up' : dir)
    const hit = hits[this.index]
    if (!hit) {
      this.term.clearSelection()
      return { at: 0, total: 0 }
    }
    this.term.select(hit.col, hit.row, hit.len)
    this.term.scrollToLine(Math.max(0, hit.row - Math.floor(this.term.rows / 2)))
    return { at: this.index + 1, total: hits.length }
  }

  clear() {
    this.query = ''
    this.index = -1
    this.term.clearSelection()
  }
}
