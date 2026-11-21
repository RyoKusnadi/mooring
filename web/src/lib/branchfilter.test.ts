import { describe, expect, it } from 'vitest'
import { branchFailureText, canCreate, filterBranches } from './branchfilter'

const names = ['main', 'feature/one', 'fix-main-menu', 'Release']

describe('filterBranches', () => {
  it('keeps everything for an empty query', () => expect(filterBranches(names, '  ')).toEqual(names))
  it('matches anywhere and ignores case, prefix matches first', () => {
    expect(filterBranches(names, 'MAIN')).toEqual(['main', 'fix-main-menu'])
    expect(filterBranches(names, 'rel')).toEqual(['Release'])
    expect(filterBranches(names, 'zzz')).toEqual([])
  })
})

describe('canCreate', () => {
  it('offers a new name only when it is not taken', () => {
    expect(canCreate(names, 'main')).toBe(false)
    expect(canCreate(names, 'mai')).toBe(true)
    expect(canCreate(names, '  ')).toBe(false)
  })
})

it('maps a git failure to plain words and never shows the raw line', () => {
  const text = branchFailureText({ code: 'GIT_FAILED', message: 'error: Your local changes to the following files would be overwritten' })
  expect(text).toContain('uncommitted changes')
  expect(text).not.toContain('error:')
  expect(branchFailureText({ code: 'X', message: 'Nope\nmore' })).toBe('Nope')
  expect(branchFailureText(null)).toBe('')
})
