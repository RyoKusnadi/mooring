import { describe, expect, it } from 'vitest'
import { MdStream, segmentEnd } from './mdstream'
import { parseMarkdown } from './md'

describe('segmentEnd', () => {
  it('stays open until a blank line follows text', () => {
    expect(segmentEnd('abc')).toBe(-1)
    expect(segmentEnd('abc\n')).toBe(-1)
    expect(segmentEnd('abc\n\ndef')).toBe(5)
  })
  it('does not split inside a code fence', () => {
    expect(segmentEnd('```\na\n\nb\n```\n\nnext')).toBe(14)
    expect(segmentEnd('```\na\n\nb')).toBe(-1)
  })
})

describe('MdStream', () => {
  const reply = '# Title\n\nFirst *para*.\n\n```go\nx := 1\n\ny := 2\n```\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\nLast words'
  it('gives the same blocks as parsing everything at once, whatever the chunking', () => {
    for (const step of [1, 3, 7, 50]) {
      const s = new MdStream()
      for (let n = step; n < reply.length + step; n += step) s.update(reply.slice(0, Math.min(n, reply.length)))
      const got = [...s.final.flatMap((x) => x.blocks), ...s.tail.blocks]
      expect(got).toEqual(parseMarkdown(reply))
    }
  })
  it('keeps final segments as the same objects while the tail grows', () => {
    const s = new MdStream()
    s.update('one\n\ntwo')
    const first = s.final[0]
    s.update('one\n\ntwo more\n\nthree')
    expect(s.final[0]).toBe(first)
    expect(s.final.map((x) => x.id)).toEqual([1, 2])
  })
  it('starts over when the text is replaced', () => {
    const s = new MdStream()
    s.update('one\n\ntwo\n\nthree')
    s.update('other\n\nthing')
    expect(s.final).toHaveLength(1)
  })
  it('re-parses only the tail for a long reply', () => {
    const para = 'A paragraph of *words* and `code` to fill space.\n\n'
    const s = new MdStream()
    let text = ''
    const t = performance.now()
    for (let i = 0; i < 4000; i++) {
      text += para
      s.update(text + 'tail')
    }
    expect(performance.now() - t).toBeLessThan(2000)
    expect(s.final).toHaveLength(4000)
  })
  it('streams 2000 lines with no blank line quickly', () => {
    const s = new MdStream()
    let text = ''
    const t = performance.now()
    for (let i = 0; i < 2000; i++) {
      text += `line ${i} of a long paragraph with *some* words\n`
      s.update(text)
    }
    expect(performance.now() - t).toBeLessThan(500)
  })
})
