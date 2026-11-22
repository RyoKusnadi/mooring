import { describe, expect, it } from 'vitest'
import { changedFiles, countChange, editsOf, lastLines, outputText, shortPath, stateLabel, toolLine } from './chattool'
import type { ChatMessage } from './chatwire'

const msg = (m: Record<string, unknown>) => ({ kind: 'tool', ...m }) as unknown as ChatMessage
const edit = msg({ toolKind: 'edit', status: 'completed', title: 'Edit', content: [{ type: 'diff', path: '/p/src/a.ts', oldText: 'a\nb\nc\n', newText: 'a\nB\nB2\nc\n' }] })

describe('running command lines', () => {
  it('draws nothing for a command that printed nothing', () => {
    const silent = msg({ toolKind: 'execute', status: 'completed', content: [{ type: 'terminal', terminalId: 'b' }] })
    expect(outputText(silent)).toBe('')
    expect(outputText(msg({ toolKind: 'execute', content: [{ type: 'content', content: { type: 'text', text: '' } }] }))).toBe('')
  })

  it('does not repeat the status word the line already starts with', () => {
    expect(stateLabel('Running make build', 'Running')).toBe('')
    expect(stateLabel('Tool call', 'Running')).toBe('Running')
    expect(stateLabel('x', undefined)).toBe('')
  })
})

describe('chattool', () => {
  it('shortens a path inside the folder', () => {
    expect(shortPath('/p/src/a.ts', '/p')).toBe('src/a.ts')
    expect(shortPath('/q/a.ts', '/p')).toBe('/q/a.ts')
  })
  it('counts lines added and removed', () => {
    expect(countChange('a\nb\nc\n', 'a\nB\nB2\nc\n')).toEqual({ added: 2, removed: 1 })
    expect(countChange('', 'x\ny')).toEqual({ added: 2, removed: 0 })
  })
  it('names an edit with its size', () => {
    expect(editsOf(edit, '/p')).toEqual([{ path: 'src/a.ts', added: 2, removed: 1 }])
    expect(toolLine(edit, '/p')).toBe('Edited src/a.ts +2 −1')
  })
  it('says failed on a failed call', () => {
    expect(toolLine(msg({ toolKind: 'execute', status: 'failed', rawInput: { command: 'npm test' } }))).toContain(', failed')
  })
  it('keeps the last 10 lines of a command', () => {
    const text = Array.from({ length: 14 }, (_, i) => 'l' + i).join('\n')
    const r = lastLines(text)
    expect(r.lines).toHaveLength(10)
    expect(r.lines[9]).toBe('l13')
    expect(r.more).toBe(true)
    expect(lastLines('one\n').more).toBe(false)
  })
  it('reads the text of a command', () => {
    expect(outputText(msg({ content: [{ type: 'content', content: { type: 'text', text: 'hi' } }] }))).toBe('hi')
  })
  it('lists each changed file once and skips failed calls', () => {
    const failed = msg({ ...edit, status: 'failed' })
    expect(changedFiles([edit, edit, failed, undefined], '/p')).toEqual(['src/a.ts'])
  })
  it('strips colour codes from the command tail', () => {
    expect(lastLines('\u001b[32mok\u001b[0m a\nb').lines).toEqual(['ok a', 'b'])
  })
})
