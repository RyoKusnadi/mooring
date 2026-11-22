import { describe, expect, it } from 'vitest'
import { findLinks, projectPath } from './termlinks'

const pick = (s: string) => findLinks(s).map((l) => l.url ?? `${l.path}${l.line ? `@${l.line}` : ''}`)

describe('findLinks', () => {
  it('finds web addresses without the punctuation after them', () => {
    expect(pick('see https://example.com/a?b=1. and (http://x.io/y)')).toEqual(['https://example.com/a?b=1', 'http://x.io/y'])
  })

  it('finds paths with a line and column, as compilers print them', () => {
    expect(pick('internal/chat/relay.go:42:7: undefined: x')).toEqual(['internal/chat/relay.go@42'])
    expect(pick('  at ./src/App.svelte:10')).toEqual(['./src/App.svelte@10'])
    expect(pick('main.go:3: syntax error')).toEqual(['main.go@3'])
  })

  it('finds paths with a slash or a plain file name, and skips versions and sentences', () => {
    expect(pick('modified:   web/src/lib/wake.ts')).toEqual(['web/src/lib/wake.ts'])
    expect(pick('edit README.md now')).toEqual(['README.md'])
    expect(pick('go 1.25.3 is fine, e.g. this. Done.')).toEqual([])
  })

  it('gives the index range of each link', () => {
    const [l] = findLinks('x a/b.ts:3 y')
    expect([l.start, l.end, l.text]).toEqual([2, 10, 'a/b.ts:3'])
  })
})

describe('projectPath', () => {
  it('joins a relative path to the folder the shell started in', () => {
    expect(projectPath('src/a.ts', '', '/p')).toBe('src/a.ts')
    expect(projectPath('./a.ts', '.mooring/worktrees/w', '/p')).toBe('.mooring/worktrees/w/a.ts')
    expect(projectPath('../x/a.ts', 'web', '/p')).toBe('x/a.ts')
  })

  it('cuts the project folder off an absolute path, and refuses paths outside', () => {
    expect(projectPath('/home/me/p/src/a.ts', '', '/home/me/p')).toBe('src/a.ts')
    expect(projectPath('/home/me/other/a.ts', '', '/home/me/p')).toBeNull()
    expect(projectPath('../../a.ts', 'web', '/p')).toBeNull()
  })
})
