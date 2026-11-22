import { Text } from '@codemirror/state'
import { describe, expect, it } from 'vitest'
import { formatChanges } from './lspformat'

const doc = Text.of(['ab  ', 'cd', ''])
const at = (line: number, character: number) => ({ line, character })

describe('formatChanges', () => {
  it('turns in-range edits into changes', () => {
    expect(formatChanges(doc, [{ range: { start: at(0, 2), end: at(0, 4) }, newText: '' }])).toEqual([{ from: 2, to: 4, insert: '' }])
  })
  it('refuses a line past the end', () => {
    expect(formatChanges(doc, [{ range: { start: at(9, 0), end: at(9, 1) }, newText: '' }])).toBeNull()
  })
  it('refuses a character past the end of its line', () => {
    expect(formatChanges(doc, [{ range: { start: at(1, 1), end: at(1, 5) }, newText: '' }])).toBeNull()
  })
  it('refuses a reversed, overlapping or malformed edit', () => {
    expect(formatChanges(doc, [{ range: { start: at(0, 3), end: at(0, 1) }, newText: '' }])).toBeNull()
    expect(
      formatChanges(doc, [
        { range: { start: at(0, 0), end: at(0, 3) }, newText: '' },
        { range: { start: at(0, 2), end: at(0, 4) }, newText: '' },
      ]),
    ).toBeNull()
    expect(formatChanges(doc, [{ range: { start: at(0, 0) }, newText: '' }])).toBeNull()
    expect(formatChanges(doc, null)).toBeNull()
  })
  it('accepts no edits', () => {
    expect(formatChanges(doc, [])).toEqual([])
  })
})
