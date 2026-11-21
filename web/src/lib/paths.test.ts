import { describe, expect, it } from 'vitest'
import { baseName, copyName, fileLabels, formatBytes, isInside, joinPath, nameProblem, parentPath, rebase } from './paths'

describe('paths', () => {
  it('joins, splits and tests containment', () => {
    expect(joinPath('', 'a.go')).toBe('a.go')
    expect(joinPath('src', 'a.go')).toBe('src/a.go')
    expect(baseName('src/lib/a.go')).toBe('a.go')
    expect(baseName('a.go')).toBe('a.go')
    expect(parentPath('src/lib/a.go')).toBe('src/lib')
    expect(parentPath('a.go')).toBe('')
    expect(isInside('src/a', 'src')).toBe(true)
    expect(isInside('src', 'src')).toBe(true)
    expect(isInside('srcx/a', 'src')).toBe(false)
    expect(isInside('anything', '')).toBe(true)
  })

  it('moves a path with its folder', () => {
    expect(rebase('src/a/b.go', 'src/a', 'lib/z')).toBe('lib/z/b.go')
    expect(rebase('src/a', 'src/a', 'lib')).toBe('lib')
  })

  it('names what is wrong with a name', () => {
    expect(nameProblem('  ')).toMatch(/Type a name/)
    expect(nameProblem('a/b')).toMatch(/"\/"/)
    expect(nameProblem('..')).toMatch(/not allowed/)
    expect(nameProblem('notes.md')).toBe('')
  })

  it('adds a folder only to duplicate names', () => {
    expect(fileLabels(['a/index.ts', 'b/index.ts', 'main.ts'])).toEqual(['a/index.ts', 'b/index.ts', 'main.ts'])
    expect(fileLabels(['index.ts', 'x/index.ts'])).toEqual(['index.ts', 'x/index.ts'])
  })

  it('names a duplicate "copy", then "copy 2", before the extension', () => {
    expect(copyName('a.txt', () => false)).toBe('a copy.txt')
    expect(copyName('a.txt', (n) => n === 'a copy.txt')).toBe('a copy 2.txt')
    expect(copyName('a.tar.gz', () => false)).toBe('a.tar copy.gz')
    expect(copyName('Makefile', () => false)).toBe('Makefile copy')
    expect(copyName('.env', () => false)).toBe('.env copy')
  })

  it('formats sizes', () => {
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2.0 KB')
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB')
  })
})
