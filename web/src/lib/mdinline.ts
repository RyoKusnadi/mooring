// Inline markdown: code spans, emphasis, strong, strikethrough, links, hard breaks. Output is a tree of
// plain data; nothing here produces HTML, so what the agent writes can only ever become text nodes.

export type Inline =
  | { t: 'text'; s: string }
  | { t: 'code'; s: string }
  | { t: 'em' | 'strong' | 'del'; c: Inline[] }
  | { t: 'link'; href: string; c: Inline[] }
  | { t: 'br' }

const PUNCT = /[!-/:-@[-`{-~]/
const ENTITIES: Record<string, string> = { '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'", '&nbsp;': ' ' }
/** Runs a sticky regex at `i` without slicing the string (slicing per character would make long text quadratic). */
function at(re: RegExp, s: string, i: number): RegExpExecArray | null {
  re.lastIndex = i
  return re.exec(s)
}
const TICKS = /`+/y
const STARS = /\*+|_+|~+/y
const ENTITY = /&(?:amp|lt|gt|quot|nbsp|#39);/y
const ANGLE = /<(https?:\/\/[^\s<>]+|mailto:[^\s<>]+)>/y
const isSpace = (c: string | undefined) => c === undefined || /\s/.test(c)
const isWord = (c: string | undefined) => c !== undefined && /[\p{L}\p{N}]/u.test(c)

/** Index just past the `]` that closes the `[` at `from`, or -1. Code spans and escapes are skipped. */
function closeBracket(s: string, from: number): number {
  let depth = 0
  for (let i = from; i < s.length; i++) {
    const c = s[i]
    if (c === '\\') i++
    else if (c === '`') {
      const run = at(TICKS, s, i)![0]
      const end = s.indexOf(run, i + run.length)
      if (end > 0) i = end + run.length - 1
    } else if (c === '[') depth++
    else if (c === ']' && --depth === 0) return i
  }
  return -1
}

/** The destination in `(url "title")` starting at `from` (the "("), and the index after ")". */
function destination(s: string, from: number): { href: string; end: number } | null {
  let i = from + 1
  while (s[i] === ' ') i++
  let href = ''
  if (s[i] === '<') {
    const e = s.indexOf('>', i)
    if (e < 0 || s.slice(i, e).includes('\n')) return null
    href = s.slice(i + 1, e)
    i = e + 1
  } else {
    let depth = 0
    const start = i
    for (; i < s.length; i++) {
      const c = s[i]
      if (c === '\\') i++
      else if (c === '(') depth++
      else if (c === ')') {
        if (depth === 0) break
        depth--
      } else if (c === ' ' || c === '\n') break
    }
    href = s.slice(start, i)
  }
  while (s[i] === ' ') i++
  const q = s[i]
  if (q === '"' || q === "'") {
    const e = s.indexOf(q, i + 1)
    if (e < 0) return null
    i = e + 1
    while (s[i] === ' ') i++
  }
  return s[i] === ')' ? { href: href.replace(/\\([!-/:-@[-`{-~])/g, '$1'), end: i + 1 } : null
}

/** The next closer for a delimiter, at or after `from`, that is a valid right-flanking run. */
function findCloser(s: string, from: number, d: string): number {
  for (let i = from; i < s.length; i++) {
    const c = s[i]
    if (c === '\\') i++
    else if (c === '`') {
      const run = at(TICKS, s, i)![0]
      const end = s.indexOf(run, i + run.length)
      if (end > 0) i = end + run.length - 1
      else i += run.length - 1
    } else if (s.startsWith(d, i) && !isSpace(s[i - 1]) && i > from) {
      if (d[0] === '_' && isWord(s[i + d.length])) continue
      if (d.length === 1 && s[i + 1] === d) continue
      return i
    }
  }
  return -1
}

const AUTOLINK = /https?:\/\/[^\s<>`]*[^\s<>`.,;:!?'")\]]/y

export function parseInline(s: string): Inline[] {
  const out: Inline[] = []
  let buf = ''
  const flush = () => {
    if (buf) out.push({ t: 'text', s: buf })
    buf = ''
  }
  for (let i = 0; i < s.length; ) {
    const c = s[i]
    if (c === '\\' && i + 1 < s.length && PUNCT.test(s[i + 1])) {
      buf += s[i + 1]
      i += 2
    } else if (c === '\\' && s[i + 1] === '\n') {
      flush()
      out.push({ t: 'br' })
      i += 2
    } else if (c === '\n') {
      let e = buf.length
      while (e > 0 && buf[e - 1] === ' ') e--
      const hard = buf.length - e >= 2
      buf = buf.slice(0, e)
      if (hard) {
        flush()
        out.push({ t: 'br' })
      } else buf += '\n'
      i++
      while (s[i] === ' ') i++
    } else if (c === '`') {
      const run = at(TICKS, s, i)![0]
      const end = s.indexOf(run, i + run.length)
      if (end < 0) {
        buf += run
        i += run.length
      } else {
        flush()
        let code = s.slice(i + run.length, end).replace(/\n/g, ' ')
        if (code.length > 2 && code.startsWith(' ') && code.endsWith(' ') && code.trim()) code = code.slice(1, -1)
        out.push({ t: 'code', s: code })
        i = end + run.length
      }
    } else if (c === '[' || (c === '!' && s[i + 1] === '[')) {
      const image = c === '!'
      const open = image ? i + 1 : i
      const close = closeBracket(s, open)
      const dest = close > 0 && s[close + 1] === '(' ? destination(s, close + 1) : null
      if (!dest) {
        buf += c
        i++
        continue
      }
      flush()
      const inner = s.slice(open + 1, close)
      // An image is never loaded; its description stays as text.
      if (image) out.push(...parseInline(inner || dest.href))
      else out.push({ t: 'link', href: dest.href, c: parseInline(inner) })
      i = dest.end
    } else if (c === '<' && at(ANGLE, s, i)) {
      const m = at(ANGLE, s, i)!
      flush()
      out.push({ t: 'link', href: m[1], c: [{ t: 'text', s: m[1] }] })
      i += m[0].length
    } else if (c === 'h' && (i === 0 || !isWord(s[i - 1])) && at(AUTOLINK, s, i)) {
      const url = at(AUTOLINK, s, i)![0]
      flush()
      out.push({ t: 'link', href: url, c: [{ t: 'text', s: url }] })
      i += url.length
    } else if (c === '&') {
      const m = at(ENTITY, s, i)
      if (m) {
        buf += ENTITIES[m[0]]
        i += m[0].length
      } else buf += s[i++]
    } else if (c === '*' || c === '_' || c === '~') {
      const run = at(STARS, s, i)![0]
      const tries = c === '~' ? (run.length >= 2 ? ['~~'] : []) : run.length >= 3 ? [c.repeat(3), c.repeat(2), c] : run.length === 2 ? [c.repeat(2), c] : [c]
      let done = false
      for (const d of tries) {
        const after = s[i + d.length]
        const before = s[i - 1]
        if (isSpace(after) || (c === '_' && isWord(before))) continue
        const end = findCloser(s, i + d.length, d)
        if (end < 0) continue
        flush()
        const c1 = parseInline(s.slice(i + d.length, end))
        if (d.length === 3) out.push({ t: 'strong', c: [{ t: 'em', c: c1 }] })
        else out.push({ t: d === '~~' ? 'del' : d.length === 2 ? 'strong' : 'em', c: c1 })
        i = end + d.length
        done = true
        break
      }
      if (!done) {
        buf += run
        i += run.length
      }
    } else {
      buf += c
      i++
    }
  }
  flush()
  return out
}
