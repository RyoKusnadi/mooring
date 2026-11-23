import { describe, expect, it } from 'vitest'
import type { Change, Status } from './gitapi'
import { letterTone, markChanges } from './treemarks'

const change = (path: string, over: Partial<Change> = {}): Change => ({ path, staged: '', unstaged: 'modified', kind: '', ...over })
const status = (files: Change[]): Status => ({ repo: true, files }) as Status

describe('markChanges', () => {
  it('marks files by the side still to work on and their folders', () => {
    const m = markChanges(status([change('src/lib/a.ts'), change('b.txt', { unstaged: '', staged: 'added' }), change('c.txt', { unstaged: '', kind: 'untracked' }), change('d.txt', { kind: 'conflicted' })]))
    expect(m.files.get('src/lib/a.ts')).toBe('M')
    expect(m.files.get('b.txt')).toBe('A')
    expect(m.files.get('c.txt')).toBe('?')
    expect(m.files.get('d.txt')).toBe('U')
    expect([...m.dirs].sort()).toEqual(['src', 'src/lib'])
  })

  it('marks nothing without a repository or changes', () => {
    expect(markChanges(null).files.size).toBe(0)
    expect(markChanges({ repo: false, files: [] } as unknown as Status).files.size).toBe(0)
    expect(markChanges(status([])).dirs.size).toBe(0)
  })

  it('tints new files green and gone ones red, and leaves edits plain', () => {
    expect(['A', '?', 'D', 'U', 'M', 'R'].map(letterTone)).toEqual(['new', 'new', 'gone', 'gone', 'edit', 'edit'])
  })
})
