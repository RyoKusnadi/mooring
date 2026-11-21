import { describe, expect, it } from 'vitest'
import { suggestSmall } from './settingsapi'

const list = [
  { id: 'default', name: 'Default' },
  { id: 'sonnet', name: 'Sonnet' },
  { id: 'haiku', name: 'Haiku' },
]

describe('suggestSmall', () => {
  it('suggests the Haiku model for Claude, by id or by name', () => {
    expect(suggestSmall('claude', list)?.id).toBe('haiku')
    expect(suggestSmall('claude', [{ id: 'x1', name: 'Claude HAIKU 9' }])?.id).toBe('x1')
  })

  it('suggests nothing when Claude offers no Haiku', () => {
    expect(suggestSmall('claude', list.slice(0, 2))).toBeUndefined()
    expect(suggestSmall('claude', [])).toBeUndefined()
  })

  it('suggests nothing for other agents, even with a haiku in the list', () => {
    expect(suggestSmall('gemini', list)).toBeUndefined()
    expect(suggestSmall('cursor', list)).toBeUndefined()
  })
})
