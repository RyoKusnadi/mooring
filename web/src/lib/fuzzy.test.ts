import { describe, expect, it } from 'vitest'
import { rankPaths, segments } from './fuzzy'

const paths = ['src/lib/files.ts', 'src/FileSearch.svelte', 'README.md', 'docs/files-api.md', 'web/e2e/files.spec.ts', 'internal/files/list.go']

describe('rankPaths', () => {
  it('puts a match in the file name before a match in a folder', () => {
    const r = rankPaths(paths, 'files')
    expect(r[0]).toBe('src/lib/files.ts')
    expect(r).toContain('internal/files/list.go')
    expect(r.indexOf('internal/files/list.go')).toBeGreaterThan(r.indexOf('docs/files-api.md'))
  })
  it('needs every word, in any order', () => {
    expect(rankPaths(paths, 'spec e2e')).toEqual(['web/e2e/files.spec.ts'])
    expect(rankPaths(paths, 'md docs')).toEqual(['docs/files-api.md'])
  })
  it('matches letters in order and ignores case', () => {
    expect(rankPaths(paths, 'FSV')).toEqual(['src/FileSearch.svelte'])
    expect(rankPaths(paths, 'readme')).toEqual(['README.md'])
  })
  it('returns nothing for a miss and the first paths for an empty query', () => {
    expect(rankPaths(paths, 'zzz')).toEqual([])
    expect(rankPaths(paths, '', 2)).toEqual(['README.md', 'src/lib/files.ts'])
  })
})

describe('rankPaths with recent files and path length', () => {
  const list = ['packages/deep/nested/util/index.ts', 'src/index.ts', 'lib/index.ts', 'src/other/index.ts']
  it('puts shorter paths first when matches are equal', () => {
    expect(rankPaths(list, 'index')).toEqual(['lib/index.ts', 'src/index.ts', 'src/other/index.ts', 'packages/deep/nested/util/index.ts'])
  })
  it('puts recently opened files before shorter ones', () => {
    const r = rankPaths(list, 'index', 50, ['src/other/index.ts', 'packages/deep/nested/util/index.ts'])
    expect(r.slice(0, 2)).toEqual(['src/other/index.ts', 'packages/deep/nested/util/index.ts'])
  })
  it('keeps a better match above a recent worse one', () => {
    expect(rankPaths(['a/notes.md', 'a/n-o-t-e-s.md'], 'notes', 50, ['a/n-o-t-e-s.md'])[0]).toBe('a/notes.md')
  })
  it('lists recent files first for an empty query, then shorter paths', () => {
    expect(rankPaths(list, '', 50, ['src/other/index.ts'])).toEqual(['src/other/index.ts', 'lib/index.ts', 'src/index.ts', 'packages/deep/nested/util/index.ts'])
  })
})

describe('segments', () => {
  it('splits a line around its ranges', () => {
    expect(segments('a foo b foo', [[2, 5], [8, 11]])).toEqual([
      { text: 'a ', hit: false },
      { text: 'foo', hit: true },
      { text: ' b ', hit: false },
      { text: 'foo', hit: true },
    ])
  })
  it('drops ranges that overlap or run past the text', () => {
    expect(segments('abc', [[0, 2], [1, 3], [2, 9]])).toEqual([
      { text: 'ab', hit: true },
      { text: 'c', hit: false },
    ])
    expect(segments('abc', [])).toEqual([{ text: 'abc', hit: false }])
  })
})
