// Links in terminal output: web addresses, and file paths with or without :line(:col).
// Plain functions; termlinkprovider.ts hands them to xterm.

export interface TermLink {
  /** Index in the line's text, end exclusive. */
  start: number
  end: number
  text: string
  url?: string
  path?: string
  line?: number
}

const URL_RE = /\bhttps?:\/\/[^\s<>"'`]+/g
// A path has a slash or ends in a file extension, and may carry :line and :col.
const PATH_RE = /(?:\.{1,2}\/|\/|\b)[\w.@+-]+(?:\/[\w.@+-]+)*(?::(\d+))?(?::\d+)?/g
const TRAIL = /[.,;:!?)\]}'"]+$/

export function findLinks(text: string): TermLink[] {
  const out: TermLink[] = []
  for (const m of text.matchAll(URL_RE)) {
    const url = m[0].replace(TRAIL, '')
    out.push({ start: m.index, end: m.index + url.length, text: url, url })
  }
  const taken = (i: number) => out.some((l) => l.url && i >= l.start && i < l.end)
  for (const m of text.matchAll(PATH_RE)) {
    if (taken(m.index)) continue
    const raw = m[0].replace(TRAIL, '')
    const at = /^(.*?)(?::(\d+))?(?::\d+)?$/.exec(raw)
    const path = at?.[1] ?? raw
    const line = at?.[2] ? Number(at[2]) : undefined
    const hasExt = /\.[A-Za-z][\w]{0,9}$/.test(path) && !/^\.+$/.test(path)
    if (!hasExt || (!path.includes('/') && line === undefined && !/^[\w-]{2,}\.[A-Za-z]{2,5}$/.test(path))) continue
    if (/^\d+(\.\d+)+$/.test(path)) continue // a version number, not a file
    out.push({ start: m.index, end: m.index + raw.length, text: raw, path, ...(line ? { line } : {}) })
  }
  return out.sort((a, b) => a.start - b.start)
}

/**
 * The project-relative path a terminal path names. `folder` is where the shell started (relative to the
 * project), `root` the project's absolute folder. Paths outside the project give null.
 */
export function projectPath(path: string, folder: string, root: string): string | null {
  let rel: string
  if (path.startsWith('/')) {
    const base = root.replace(/\/+$/, '')
    if (!base || !path.startsWith(base + '/')) return null
    rel = path.slice(base.length + 1)
  } else {
    rel = folder ? `${folder}/${path}` : path
  }
  const parts: string[] = []
  for (const p of rel.split('/')) {
    if (p === '' || p === '.') continue
    if (p === '..') {
      if (!parts.length) return null
      parts.pop()
    } else parts.push(p)
  }
  return parts.length ? parts.join('/') : null
}
