import { describe, expect, it } from 'vitest'
import { icons } from './icons'
import { tabIcon } from './tabicon'

// Step 5 of the visual plan: one set, every name present.
const SET = ['plus', 'x', 'minus', 'chevron', 'folder', 'file', 'search', 'sliders', 'server', 'plug', 'user', 'logout', 'diff', 'terminal', 'chat']

describe('icons', () => {
  it('has the whole Step 5 set, each with a path', () => {
    for (const n of SET) expect(icons[n as keyof typeof icons], n).toMatch(/^M/)
  })
  it('gives every tab kind an icon that exists', () => {
    for (const k of ['terminal', 'chat', 'file', 'rfile', 'diff', 'commit', 'history', 'remote', 'blank']) expect(icons[tabIcon(k)]).toBeTruthy()
    expect(tabIcon('terminal')).toBe('terminal')
    expect(tabIcon('diff')).toBe('diff')
  })
})
