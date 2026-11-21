import { describe, expect, it } from 'vitest'
import { tabLabels } from './tablabels'

describe('tabLabels', () => {
  it('shows the same chat title as the chat header, New chat when there is none', () => {
    const tabs = [{ id: 'a', kind: 'chat', title: 'Chat' }, { id: 'b', kind: 'chat', title: '' }]
    const out = tabLabels(tabs, (id) => (id === 'a' ? 'Fix the build' : undefined))
    expect(out.get('a')).toBe('Fix the build')
    expect(out.get('b')).toBe('New chat')
  })
  it('numbers equal labels and names an empty tab Blank', () => {
    const tabs = [{ id: 'a', kind: 'blank', title: '' }, { id: 'b', kind: 'blank', title: '' }]
    const out = tabLabels(tabs, () => undefined)
    expect([out.get('a'), out.get('b')]).toEqual(['Blank', 'Blank 2'])
  })
  it('puts the host name in front on a remote tab, once', () => {
    const tabs = [{ id: 'a', kind: 'terminal', title: 'Build terminal', host: 'h1' }, { id: 'b', kind: 'terminal', title: 'x', host: 'h1' }, { id: 'c', kind: 'terminal', title: 'x' }]
    const out = tabLabels(tabs, (id) => (id === 'b' ? 'e2e@box: ~' : undefined), (h) => (h === 'h1' ? 'Build' : undefined))
    expect([out.get('a'), out.get('b'), out.get('c')]).toEqual(['Build terminal', 'Build: e2e@box: ~', 'x'])
  })
})
