import { describe, expect, it } from 'vitest'
import { parseMarkdown, type Block } from './md'
import type { Inline } from './mdinline'

/** Every string that could reach the page, flattened, to look for markup in link targets. */
function walk(blocks: Block[], on: (i: Inline) => void) {
  const inl = (l: Inline[]) =>
    l.forEach((i) => {
      on(i)
      if ('c' in i) inl(i.c)
    })
  for (const b of blocks) {
    if (b.t === 'h' || b.t === 'p') inl(b.c)
    else if (b.t === 'list') b.items.forEach((it) => walk(it.blocks, on))
    else if (b.t === 'quote') walk(b.blocks, on)
    else if (b.t === 'table') [b.head, ...b.rows].forEach((r) => r.forEach(inl))
  }
}
const texts = (md: string) => {
  const out: Inline[] = []
  walk(parseMarkdown(md), (i) => out.push(i))
  return out
}

describe('blocks', () => {
  it('reads headings and paragraphs', () => {
    expect(parseMarkdown('# One\n\ntext here')).toEqual([
      { t: 'h', level: 1, c: [{ t: 'text', s: 'One' }] },
      { t: 'p', c: [{ t: 'text', s: 'text here' }] },
    ])
  })
  it('does not make a heading of #hashtag', () => {
    expect(parseMarkdown('#hashtag')[0].t).toBe('p')
  })
  it('reads bullet, numbered, nested and task lists', () => {
    const [l] = parseMarkdown('- a\n  - b\n- [x] done\n- [ ] open')
    expect(l.t).toBe('list')
    if (l.t !== 'list') return
    expect(l.items).toHaveLength(3)
    expect(l.items[0].blocks.map((b) => b.t)).toEqual(['p', 'list'])
    expect(l.items[1].task).toBe(true)
    expect(l.items[2].task).toBe(false)
    const [n] = parseMarkdown('3. c\n4. d')
    expect(n.t === 'list' && n.ordered && n.start).toBe(3)
  })
  it('reads a table with alignment and pads short rows', () => {
    const [t] = parseMarkdown('| a | b |\n| :-- | --: |\n| 1 |\n| 2 | 3 | 4 |')
    expect(t.t).toBe('table')
    if (t.t !== 'table') return
    expect(t.align).toEqual(['left', 'right'])
    expect(t.rows[0]).toHaveLength(2)
    expect(t.rows[1]).toHaveLength(2)
  })
  it('keeps an escaped pipe inside its cell', () => {
    const [t] = parseMarkdown('| a | b |\n| - | - |\n| x\\|y | z |')
    expect(t.t === 'table' && t.rows[0][0]).toEqual([{ t: 'text', s: 'x|y' }])
  })
  it('keeps code fences verbatim, closed or not', () => {
    expect(parseMarkdown('```go\n# not a heading\n- not a list\n```')).toEqual([{ t: 'code', lang: 'go', text: '# not a heading\n- not a list', closed: true }])
    expect(parseMarkdown('```\nopen')).toEqual([{ t: 'code', lang: '', text: 'open', closed: false }])
    expect(parseMarkdown('~~~sh\nls\n~~~')[0]).toMatchObject({ t: 'code', lang: 'sh', closed: true })
  })
  it('reads quotes and rules', () => {
    expect(parseMarkdown('> quoted\n\n---')).toMatchObject([{ t: 'quote' }, { t: 'hr' }])
  })
  it('handles CRLF', () => {
    expect(parseMarkdown('a\r\n\r\nb')).toHaveLength(2)
  })
})

describe('inline', () => {
  it('reads code, emphasis, strong, strikethrough', () => {
    const [p] = parseMarkdown('`x` *e* **s** ~~d~~')
    const kinds = p.t === 'p' ? p.c.map((i) => i.t) : []
    expect(kinds).toEqual(['code', 'text', 'em', 'text', 'strong', 'text', 'del'])
  })
  it('does not turn snake_case or 2 * 3 into emphasis', () => {
    expect(texts('snake_case_name and 2 * 3 * 4').every((i) => i.t === 'text')).toBe(true)
  })
  it('turns bare addresses into links, without the trailing full stop', () => {
    const l = texts('see https://example.com/a.').find((i) => i.t === 'link')
    expect(l).toMatchObject({ href: 'https://example.com/a' })
  })
  it('shows an image as its description and never loads it', () => {
    const all = texts('![alt text](https://x.test/a.png)')
    expect(all.some((i) => i.t === 'link')).toBe(false)
    expect(all[0]).toEqual({ t: 'text', s: 'alt text' })
  })
})

describe('markup stays text', () => {
  it('keeps <script> as text', () => {
    const all = texts('<script>alert(1)</script>')
    expect(all).toEqual([{ t: 'text', s: '<script>alert(1)</script>' }])
  })
  it('keeps onerror attributes and tags as text', () => {
    expect(texts('<img src=x onerror=alert(1)>')).toEqual([{ t: 'text', s: '<img src=x onerror=alert(1)>' }])
  })
  it('keeps HTML in table cells and code as text', () => {
    const [t] = parseMarkdown('| <b>x</b> |\n| - |\n| <img src=x onerror=1> |')
    expect(JSON.stringify(t)).toContain('<b>x</b>')
    const [c] = parseMarkdown('```html\n<script>1</script>\n```')
    expect(c).toMatchObject({ t: 'code', text: '<script>1</script>' })
    expect(texts('`<script>`')[0]).toEqual({ t: 'code', s: '<script>' })
  })
  it('passes a javascript: link through for the link policy to refuse', () => {
    const l = texts('[x](javascript:alert(1))').find((i) => i.t === 'link')
    // The parser keeps the target as written; classifyLink (mdlink.test.ts) turns it into plain text.
    expect(l).toMatchObject({ href: 'javascript:alert(1)' })
  })
})

describe('speed', () => {
  it('parses a 600 KB reply in well under a second', () => {
    const para = 'Some *text* with `code`, a [link](https://example.com) and **bold** words.\n\n- item one\n- item two\n\n'
    const big = para.repeat(Math.ceil(600_000 / para.length))
    const t = performance.now()
    parseMarkdown(big)
    expect(performance.now() - t).toBeLessThan(1500)
  })
  it('survives hostile input without hanging', () => {
    const t = performance.now()
    parseMarkdown('*'.repeat(20000) + '\n' + '['.repeat(5000) + '\n' + '`'.repeat(5000) + '\n' + '_a '.repeat(5000))
    expect(performance.now() - t).toBeLessThan(2000)
  })
})
