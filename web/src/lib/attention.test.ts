import { describe, expect, it } from 'vitest'
import { faviconFor, needCount, needText, showAttention, titleFor } from './attention'

describe('needs you', () => {
  it('counts chats and terminals together', () => {
    expect(needCount(2, 1)).toBe(3)
    expect(needCount(0, 0)).toBe(0)
  })
  it('words the status bar line', () => {
    expect(needText(2, 0, '2 chats need permission')).toBe('2 chats need permission')
    expect(needText(0, 1, '')).toBe('1 terminal needs you')
    expect(needText(0, 3, '')).toBe('3 terminals need you')
    expect(needText(1, 1, 'x')).toBe('1 chat and 1 terminal need you')
    expect(needText(2, 2, 'x')).toBe('2 chats and 2 terminals need you')
  })
})

describe('attention outside the tab', () => {
  it('puts the count in the title', () => {
    expect(titleFor(0)).toBe('Mooring')
    expect(titleFor(2)).toBe('(2) Mooring')
  })
  it('draws the dot only when something waits', () => {
    expect(decodeURIComponent(faviconFor(0))).not.toContain('<circle')
    expect(decodeURIComponent(faviconFor(1))).toContain('<circle')
  })
  it('sets one favicon link and updates it', () => {
    const links: { rel: string; type: string; href: string }[] = []
    const doc = {
      title: '',
      head: { appendChild: (l: (typeof links)[number]) => links.push(l) },
      querySelector: () => links[0] ?? null,
      createElement: () => ({ rel: '', type: '', href: '' }),
    } as unknown as Document
    showAttention(1, doc)
    showAttention(3, doc)
    expect(doc.title).toBe('(3) Mooring')
    expect(links).toHaveLength(1)
    expect(decodeURIComponent(links[0].href)).toContain('<circle')
    showAttention(0, doc)
    expect(doc.title).toBe('Mooring')
    expect(decodeURIComponent(links[0].href)).not.toContain('<circle')
  })
})

describe('favicon rewrite', () => {
  it('writes the link only when the count changes', () => {
    let writes = 0
    const link = { rel: '', type: '', set href(_v: string) { writes++ } }
    const doc = { title: '', head: { appendChild() {} }, querySelector: () => link, createElement: () => link } as unknown as Document
    showAttention(1, doc)
    showAttention(1, doc)
    showAttention(2, doc)
    expect(writes).toBe(2)
  })
})
