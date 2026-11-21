import { describe as suite, expect, it } from 'vitest'
import { CommandError } from './events'
import { describe, fieldFor } from './errors'
import { filterItems } from './filter'

suite('fieldFor', () => {
  it('puts errors next to the field they concern', () => {
    expect(fieldFor('open', 'ALREADY_ADDED')).toBe('folder')
    expect(fieldFor('create', 'ALREADY_EXISTS')).toBe('name')
    expect(fieldFor('create', 'OUTSIDE_ROOT')).toBe('parent')
    expect(fieldFor('clone', 'CLONE_FAILED')).toBe('url')
    expect(fieldFor('clone', 'ALREADY_EXISTS', true)).toBe('name')
    expect(fieldFor('clone', 'ALREADY_EXISTS', false)).toBe('url')
    expect(fieldFor('clone', 'INTERNAL')).toBe('form')
  })
})

suite('describe', () => {
  it('uses the server message and says plainly when the outcome is unknown', () => {
    expect(describe('create', new CommandError('ALREADY_EXISTS', 'That folder already exists.'))).toEqual({
      field: 'name',
      message: 'That folder already exists.',
    })
    expect(describe('open', new CommandError('OUTCOME_UNKNOWN', 'x')).field).toBe('form')
    expect(describe('open', new Error('x')).field).toBe('form')
  })
})

suite('filterItems', () => {
  const items = [{ label: 'Split right' }, { label: 'Switch to api' }, { label: 'Sign out' }, { label: 'Close tab' }]
  it('returns everything for an empty query', () => {
    expect(filterItems(items, '  ')).toHaveLength(4)
  })
  it('needs every word, in any order', () => {
    expect(filterItems(items, 'right split').map((i) => i.label)).toEqual(['Split right'])
    expect(filterItems(items, 'zzz')).toEqual([])
  })
  it('ranks matches at the start of a word first', () => {
    expect(filterItems(items, 'si')[0].label).toBe('Sign out')
  })
})
