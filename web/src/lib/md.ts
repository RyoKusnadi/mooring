// Block markdown for chat replies: headings, paragraphs, lists (nested, task), blockquotes, fenced code,
// tables and rules. A hand-written subset; the output is plain data and never HTML.
import { parseInline, type Inline } from './mdinline'

export type { Inline } from './mdinline'

export interface Item {
  /** undefined for a plain item, else whether the task box is ticked. */
  task?: boolean
  blocks: Block[]
}
export type Block =
  | { t: 'h'; level: number; c: Inline[] }
  | { t: 'p'; c: Inline[] }
  | { t: 'list'; ordered: boolean; start: number; items: Item[] }
  | { t: 'quote'; blocks: Block[] }
  | { t: 'code'; lang: string; text: string; closed: boolean }
  | { t: 'hr' }
  | { t: 'table'; head: Inline[][]; align: ('left' | 'right' | 'center' | '')[]; rows: Inline[][][] }

const FENCE = /^ {0,3}(`{3,}|~{3,})\s*([^\s`]*)[^`]*$/
const HEADING = /^ {0,3}(#{1,6})(?:[ \t]+(.*?))?(?:[ \t]+#+)?[ \t]*$/
const RULE = /^ {0,3}([-*_])(?: *\1){2,} *$/
const ITEM = /^( {0,3})([-*+]|\d{1,9}[.)])( +|$)(.*)$/
const QUOTE = /^ {0,3}> ?/
const DELIM = /^ *\|? *:?-+:? *(?:\| *:?-+:? *)*\|? *$/

const indentOf = (l: string) => /^ */.exec(l)![0].length
const blank = (l: string) => l.trim() === ''

/** Cells of a table row; escaped pipes stay in the cell. */
function cells(line: string): string[] {
  let s = line.trim()
  if (s.startsWith('|')) s = s.slice(1)
  if (s.endsWith('|') && !s.endsWith('\\|')) s = s.slice(0, -1)
  const out: string[] = []
  let cur = ''
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '\\' && s[i + 1] === '|') (cur += '|'), i++
    else if (s[i] === '|') (out.push(cur.trim()), (cur = ''))
    else cur += s[i]
  }
  out.push(cur.trim())
  return out
}

const tableStart = (lines: string[], i: number) => i + 1 < lines.length && lines[i].includes('|') && DELIM.test(lines[i + 1]) && lines[i + 1].includes('-') && cells(lines[i]).length === cells(lines[i + 1]).length

function startsBlock(l: string): boolean {
  if (FENCE.test(l) || HEADING.test(l) || RULE.test(l) || QUOTE.test(l)) return true
  const m = ITEM.exec(l)
  // A list interrupts a paragraph only when it has text, and a number only when it is 1.
  return !!m && m[4].trim() !== '' && (/^[-*+]$/.test(m[2]) || parseInt(m[2], 10) === 1)
}

function parseList(lines: string[], from: number): { block: Block; next: number } {
  const first = ITEM.exec(lines[from])!
  const ordered = /\d/.test(first[2])
  const kind = ordered ? first[2].slice(-1) : first[2]
  const items: Item[] = []
  let i = from
  while (i < lines.length) {
    const m = ITEM.exec(lines[i])
    if (!m || /\d/.test(m[2]) !== ordered || (ordered ? m[2].slice(-1) : m[2]) !== kind) break
    const pad = m[3].length > 4 || m[3] === '' ? 1 : m[3].length
    const content = m[1].length + m[2].length + pad
    const body = [m[4]]
    i++
    for (; i < lines.length; i++) {
      const l = lines[i]
      if (blank(l)) {
        let j = i
        while (j < lines.length && blank(lines[j])) j++
        if (j < lines.length && indentOf(lines[j]) >= content) {
          for (; i < j; i++) body.push('')
          i--
          continue
        }
        break
      }
      if (indentOf(l) >= content) body.push(l.slice(content))
      else if (ITEM.test(l) || startsBlock(l) || blank(body[body.length - 1])) break
      else body.push(l.trim())
    }
    const task = /^\[([ xX])\][ \t]+/.exec(body[0])
    if (task) body[0] = body[0].slice(task[0].length)
    items.push({ task: task ? task[1] !== ' ' : undefined, blocks: parseBlocks(body) })
    while (i < lines.length && blank(lines[i]) && i + 1 < lines.length && ITEM.test(lines[i + 1]) && !blank(lines[i + 1])) i++
  }
  return { block: { t: 'list', ordered, start: ordered ? parseInt(first[2], 10) : 1, items }, next: i }
}

export function parseBlocks(lines: string[]): Block[] {
  const out: Block[] = []
  let i = 0
  while (i < lines.length) {
    const l = lines[i]
    if (blank(l)) {
      i++
      continue
    }
    const f = FENCE.exec(l)
    if (f) {
      const mark = f[1]
      const body: string[] = []
      const strip = indentOf(l)
      let closed = false
      for (i++; i < lines.length; i++) {
        const t = lines[i].trim()
        if (t.length >= mark.length && t === mark[0].repeat(t.length)) {
          closed = true
          i++
          break
        }
        body.push(lines[i].replace(new RegExp(`^ {0,${strip}}`), ''))
      }
      out.push({ t: 'code', lang: f[2].toLowerCase(), text: body.join('\n'), closed })
      continue
    }
    const h = HEADING.exec(l)
    if (h) {
      out.push({ t: 'h', level: h[1].length, c: parseInline(h[2] ?? '') })
      i++
      continue
    }
    if (RULE.test(l)) {
      out.push({ t: 'hr' })
      i++
      continue
    }
    if (QUOTE.test(l)) {
      const body: string[] = []
      for (; i < lines.length && QUOTE.test(lines[i]); i++) body.push(lines[i].replace(QUOTE, ''))
      out.push({ t: 'quote', blocks: parseBlocks(body) })
      continue
    }
    if (ITEM.test(l)) {
      const r = parseList(lines, i)
      out.push(r.block)
      i = r.next
      continue
    }
    if (tableStart(lines, i)) {
      const head = cells(l)
      const align = cells(lines[i + 1]).map((c) => (c.startsWith(':') && c.endsWith(':') ? 'center' : c.endsWith(':') ? 'right' : c.startsWith(':') ? 'left' : '') as 'left' | 'right' | 'center' | '')
      const rows: Inline[][][] = []
      for (i += 2; i < lines.length && !blank(lines[i]) && lines[i].includes('|'); i++) {
        const row = cells(lines[i])
        while (row.length < head.length) row.push('')
        rows.push(row.slice(0, head.length).map(parseInline))
      }
      out.push({ t: 'table', head: head.map(parseInline), align, rows })
      continue
    }
    const para = [l.trimStart()]
    for (i++; i < lines.length && !blank(lines[i]) && !startsBlock(lines[i]) && !tableStart(lines, i); i++) para.push(lines[i].trimStart())
    out.push({ t: 'p', c: parseInline(para.join('\n').trimEnd()) })
  }
  return out
}

export const parseMarkdown = (text: string): Block[] => parseBlocks(text.replace(/\r\n?/g, '\n').split('\n'))
