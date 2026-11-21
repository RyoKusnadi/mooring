import { describe, expect, it } from 'vitest'
import { CommandError } from './events'
import { chipText, chipTitle, costText, lineRange, outputCount, suggestError, summarizeError } from './tokenfmt'

describe('suggestError', () => {
  it.each(['NOTHING_TO_COMMIT', 'TIMEOUT', 'LIMIT_REACHED', 'BUSY', 'BAD_REQUEST'])('explains %s in words', (code) => {
    const t = suggestError(new CommandError(code, 'server text'))
    expect(t).not.toContain(code)
    expect(t).not.toBe('server text')
  })
  it('passes other messages through', () => {
    expect(suggestError(new CommandError('AGENT_FAILED', 'The agent failed.'))).toBe('The agent failed.')
  })
})

describe('summarizeError', () => {
  it('says what busy means', () => {
    expect(summarizeError(new CommandError('BUSY', 'x'))).toMatch(/reply/)
  })
})

describe('chips', () => {
  it('names the file and the lines', () => {
    expect(chipText({ kind: 'selection', path: 'src/a/b.go', from: 3, to: 9 })).toBe('b.go, lines 3 to 9')
    expect(chipText({ kind: 'selection', path: 'b.go', from: 4, to: 4 })).toBe('b.go, line 4')
    expect(chipTitle({ kind: 'selection', path: 'src/b.go', from: 3, to: 9 })).toBe('src/b.go, lines 3 to 9')
  })
  it('counts the lines of an output', () => {
    expect(chipText({ kind: 'output', outputId: 'o1', lines: 1200 })).toBe('Command output, 1,200 lines')
    expect(chipText({ kind: 'output', outputId: 'o1' })).toBe('Command output')
  })
})

describe('outputCount', () => {
  it('says how much the agent got when output was cut', () => {
    expect(outputCount({ id: 'o', lines: 12000, bytes: 1, keptLines: 120, compacted: true })).toBe('The agent got 120 of 12,000 lines')
    expect(outputCount({ id: 'o', lines: 1, bytes: 1, keptLines: 1, compacted: false })).toBe('1 line')
  })
})

describe('lineRange', () => {
  const doc = { lineAt: (pos: number) => ({ number: Math.floor(pos / 10) + 1 }) }
  it('covers the lines of a selection', () => expect(lineRange(doc, 5, 25)).toEqual({ from: 1, to: 3 }))
  it('stops before a line that is selected only by its line break', () => expect(lineRange(doc, 0, 20)).toEqual({ from: 1, to: 2 }))
  it('takes one line for a single position', () => expect(lineRange(doc, 12, 12)).toEqual({ from: 2, to: 2 }))
})

describe('costText', () => {
  it('rounds the reported cost to cents', () => {
    expect(costText(0.13050769999999998, 'USD')).toBe('$0.13')
    expect(costText(0)).toBe('$0.00')
    expect(costText(0.004, 'USD')).toBe('<$0.01')
    expect(costText(2.5, 'EUR')).toBe('€2.50')
    expect(costText(1, 'not a code')).toBe('$1.00')
  })
})
