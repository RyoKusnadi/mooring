import { describe, expect, it } from 'vitest'
import { delimiterFor, parseCsv } from './csv'

describe('parseCsv', () => {
  it('reads plain rows, with or without a final newline', () => {
    expect(parseCsv('a,b\n1,2\n').rows).toEqual([['a', 'b'], ['1', '2']])
    expect(parseCsv('a,b\n1,2').rows).toEqual([['a', 'b'], ['1', '2']])
  })
  it('handles quotes, commas, doubled quotes and line breaks inside quotes', () => {
    const r = parseCsv('name,note\n"Lee, A","said ""hi"""\n"two\nlines",x\n').rows
    expect(r).toEqual([['name', 'note'], ['Lee, A', 'said "hi"'], ['two\nlines', 'x']])
  })
  it('accepts CRLF and lone CR, and drops a byte order mark', () => {
    expect(parseCsv('﻿a,b\r\n1,2\r3,4').rows).toEqual([['a', 'b'], ['1', '2'], ['3', '4']])
  })
  it('keeps empty fields and an empty quoted field, and skips blank lines', () => {
    expect(parseCsv('a,,c\n\n"",x,\n').rows).toEqual([['a', '', 'c'], ['', 'x', '']])
  })
  it('reads tabs', () => {
    expect(parseCsv('a\tb\n1,5\t2', '\t').rows).toEqual([['a', 'b'], ['1,5', '2']])
    expect(delimiterFor('x.TSV')).toBe('\t')
    expect(delimiterFor('x.csv')).toBe(',')
  })
  it('stops at the row cap and says there was more', () => {
    const r = parseCsv('h\n1\n2\n3\n4', ',', 3)
    expect(r.rows).toEqual([['h'], ['1'], ['2']])
    expect(r.more).toBe(true)
    expect(parseCsv('h\n1\n2', ',', 3).more).toBe(false)
    expect(parseCsv('h\n1\n2\n', ',', 3).more).toBe(false)
  })
  it('does not hang on an unclosed quote', () => {
    expect(parseCsv('a,"b\nc').rows).toEqual([['a', 'b\nc']])
  })
  it('returns no rows for empty text', () => {
    expect(parseCsv('').rows).toEqual([])
  })
})
