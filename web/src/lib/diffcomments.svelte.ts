// Comments on diff lines, kept per project for this browser tab (sessionStorage) until they are sent to a chat.
import { MAX_ATTACH_BYTES } from './attachstore.svelte'

export interface DiffComment {
  id: string
  path: string
  /** The line number in the new file, or in the old file for a removed line. */
  line: number
  /** Last line of a range; the same as `line` for one line. */
  to: number
  removed: boolean
  /** The quoted line or lines, without the leading + or -. */
  quote: string
  text: string
}

const key = (project: string) => `mooring.diffcomments.${project}`

function load(project: string): DiffComment[] {
  try {
    const v: unknown = JSON.parse(sessionStorage.getItem(key(project)) ?? '[]')
    return Array.isArray(v) ? (v as DiffComment[]).filter((c) => typeof c?.path === 'string' && typeof c.text === 'string' && typeof c.line === 'number') : []
  } catch {
    return []
  }
}

/** A code fence longer than any run of backticks inside `text`. */
function fence(text: string): string {
  const longest = Math.max(2, ...(text.match(/`+/g) ?? []).map((r) => r.length))
  return '`'.repeat(longest + 1)
}

const where = (c: DiffComment) => `${c.path}:${c.line}${c.to > c.line ? `-${c.to}` : ''}${c.removed ? ' (removed line)' : ''}`

/** The message that goes into the chat's draft: each comment as path:line, the quoted lines, then the comment. */
export function commentsText(list: DiffComment[]): string {
  const sorted = [...list].sort((a, b) => (a.path === b.path ? a.line - b.line : a.path < b.path ? -1 : 1))
  const parts = sorted.map((c) => {
    const f = fence(c.quote)
    return `${where(c)}\n${f}\n${c.quote}\n${f}\n${c.text.trim()}`
  })
  return `Comments on the changes:\n\n${parts.join('\n\n')}`
}

class DiffComments {
  // A plain map holds the lists, so reading one never writes state; `version` tells readers a list changed.
  private lists = new Map<string, DiffComment[]>()
  private version = $state(0)

  list(project: string): DiffComment[] {
    void this.version
    let l = this.lists.get(project)
    if (!l) this.lists.set(project, (l = load(project)))
    return l
  }

  /** Comments of one line (or the line's range start). */
  at(project: string, path: string, line: number, removed: boolean): DiffComment | undefined {
    return this.list(project).find((c) => c.path === path && c.line === line && c.removed === removed)
  }

  /** Adds a comment, or replaces the one on the same line. */
  save(project: string, c: Omit<DiffComment, 'id'>) {
    const rest = this.list(project).filter((x) => !(x.path === c.path && x.line === c.line && x.removed === c.removed))
    this.set(project, [...rest, { ...c, id: `${Date.now()}-${rest.length}` }])
  }

  remove(project: string, id: string) {
    this.set(project, this.list(project).filter((c) => c.id !== id))
  }

  clear(project: string) {
    this.set(project, [])
  }

  /** The text to send, or an error when it is over 64 KB. */
  prepare(project: string): { text: string; error: string } {
    const text = commentsText(this.list(project))
    if (new TextEncoder().encode(text).length > MAX_ATTACH_BYTES) return { text: '', error: 'The comments are over 64 KB together. Send some now and keep the rest, or discard some.' }
    return { text, error: '' }
  }

  private set(project: string, list: DiffComment[]) {
    this.lists.set(project, list)
    this.version++
    try {
      if (list.length) sessionStorage.setItem(key(project), JSON.stringify(list))
      else sessionStorage.removeItem(key(project))
    } catch {
      // Storage full or blocked: the comments stay in memory for this page.
    }
  }
}

export const diffComments = new DiffComments()
