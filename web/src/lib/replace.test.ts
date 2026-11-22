import { describe, expect, it } from 'vitest'
import { applyEdits } from './lspedits'
import { countPlaces, matcher, planText } from './replace'

const re = (o: Parameters<typeof matcher>[0]) => {
  const m = matcher(o)
  if (!(m instanceof RegExp)) throw new Error(m.error)
  return m
}

describe('matcher', () => {
  it('escapes plain text, follows the case option, and reports a bad pattern', () => {
    expect('a.b axb A.B'.match(re({ q: 'a.b' }))).toEqual(['a.b', 'A.B'])
    expect('a.b A.B'.match(re({ q: 'a.b', matchCase: true }))).toEqual(['a.b'])
    expect(matcher({ q: '(', regex: true })).toHaveProperty('error')
    expect(matcher({ q: '' })).toHaveProperty('error')
  })
})

describe('planText', () => {
  it('replaces every match per line, keeps CRLF, and the edits give the previewed text', () => {
    const text = 'foo = foo\r\nbar\r\nfoo()\r\n'
    const p = planText('a.go', text, re({ q: 'foo', matchCase: true }), 'sum', false)
    expect(p.lines).toEqual([
      { line: 1, before: 'foo = foo', after: 'sum = sum' },
      { line: 3, before: 'foo()', after: 'sum()' },
    ])
    expect(applyEdits(text, p.edits)).toBe('sum = sum\r\nbar\r\nsum()\r\n')
    expect(countPlaces([p, p])).toBe(6)
  })

  it('takes groups in regex mode and $ literally otherwise', () => {
    const r = planText('a.ts', 'get(a, b)\n', re({ q: 'get\\((\\w+), (\\w+)\\)', regex: true }), 'get($2, $1)', true)
    expect(r.lines[0].after).toBe('get(b, a)')
    const plain = planText('a.ts', 'price\n', re({ q: 'price' }), '$1', false)
    expect(plain.lines[0].after).toBe('$1')
  })

  it('keeps anchors, word edges and lookaround when filling in the groups', () => {
    const at = (q: string, text: string, w: string) => planText('a', text + '\n', re({ q, regex: true }), w, true).lines[0]?.after
    expect(at('^(\\w+) =', 'x = x =', '$1 :=')).toBe('x := x =')
    expect(at('\\bfoo\\b', 'foo food foo', 'bar')).toBe('bar food bar')
    expect(at('(?<=\\.)(\\w+)', 'a.b c.d', '[$1]')).toBe('a.[b] c.[d]')
    expect(at('(?<k>\\w+)=1', 'a=1 b=1', '$<k>=2')).toBe('a=2 b=2')
  })

  it('records the text it planned on', () => {
    expect(planText('a', 'foo\n', re({ q: 'foo' }), 'bar', false).planned).toBe('foo\n')
  })

  it('skips empty matches', () => {
    expect(planText('a', 'abc\n', re({ q: 'x*', regex: true }), 'y', true).edits).toEqual([])
  })
})
