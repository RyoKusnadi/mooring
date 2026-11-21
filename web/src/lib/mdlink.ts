// Where a link in a chat reply may go. Only http(s) and mailto leave the page; a path opens the
// file in Mooring when it lies inside the project; everything else is shown as plain text.

export type LinkTarget =
  | { kind: 'web'; href: string }
  | { kind: 'file'; path: string; line?: number }
  | { kind: 'none' }

// eslint-disable-next-line no-control-regex
const CONTROL = /[\x00-\x20\x7f-\x9f\p{Zl}\p{Zp}]/gu

/** Cuts a path into segments, resolving "." and "..". Null when it climbs out of the root. */
function normalise(parts: string[]): string[] | null {
  const out: string[] = []
  for (const p of parts) {
    if (p === '' || p === '.') continue
    if (p === '..') {
      if (!out.pop()) return null
    } else out.push(p)
  }
  return out
}

/** `root` is the absolute path of the project folder. The path answer is relative to it, "/"-separated. */
export function classifyLink(href: string, root: string): LinkTarget {
  const raw = href.trim()
  // Browsers drop tabs and newlines inside a scheme, so "java\nscript:" must not slip by: judge the cleaned text.
  const clean = raw.replace(CONTROL, '')
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(clean)?.[1].toLowerCase()
  if (scheme === 'http' || scheme === 'https') {
    try {
      const u = new URL(clean)
      return u.hostname ? { kind: 'web', href: u.href } : { kind: 'none' }
    } catch {
      return { kind: 'none' }
    }
  }
  if (scheme === 'mailto') return /^mailto:[^\s<>]+$/i.test(clean) ? { kind: 'web', href: clean } : { kind: 'none' }
  let path = raw
  if (scheme === 'file') path = raw.replace(/^file:\/\//i, '')
  else if (scheme || raw.startsWith('//') || raw.startsWith('#') || raw === '') return { kind: 'none' }
  let line: number | undefined
  const frag = /#L?(\d+)(?:-L?\d+)?$/.exec(path)
  if (frag) line = parseInt(frag[1], 10)
  path = path.replace(/#.*$/, '').replace(/\?.*$/, '')
  const colon = /^(.*?):(\d+)(?::\d+)?$/.exec(path)
  if (colon) (path = colon[1]), (line = parseInt(colon[2], 10))
  try {
    path = decodeURIComponent(path)
  } catch {
    return { kind: 'none' }
  }
  if (path.includes('\0') || path.includes('\\')) return { kind: 'none' }
  let parts: string[]
  if (path.startsWith('/')) {
    const base = root.replace(/\/+$/, '')
    if (!base || !(path === base || path.startsWith(base + '/'))) return { kind: 'none' }
    parts = path.slice(base.length).split('/')
  } else parts = path.split('/')
  const norm = normalise(parts)
  if (!norm || norm.length === 0) return { kind: 'none' }
  return { kind: 'file', path: norm.join('/'), ...(line ? { line } : {}) }
}
