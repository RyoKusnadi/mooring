import { describe, expect, it } from 'vitest'
import { chatPlace, chatRoot } from './chatplace'
import type { ChatInfo } from './chatstore.svelte'

const info = (path?: string) => ({ spec: path ? { folder: { path } } : null }) as unknown as ChatInfo

describe('chatPlace', () => {
  it('is the worktree folder without waiting for any list', () => {
    expect(chatPlace('c', info('/w/p-wt'), '/w/p')).toBe('/w/p-wt')
    expect(chatRoot(info('/w/p-wt'), '/w/p')).toBe('/w/p-wt')
  })
  it('is the project itself for a chat in the project folder', () => {
    expect(chatPlace('c', info(), '/w/p')).toBe('')
    expect(chatPlace('c', info('/w/p/'), '/w/p')).toBe('')
    expect(chatRoot(info(), '/w/p')).toBe('/w/p')
  })
})
