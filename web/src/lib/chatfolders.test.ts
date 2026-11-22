import { describe, expect, it } from 'vitest'
import { deleteQuestion, readsProject } from './chatfolders'

describe('readsProject', () => {
  it('needs a worktree chat and an agent that takes extra folders', () => {
    const tree = { folder: { worktree: true } }
    expect(readsProject({ spec: tree, caps: { additionalDirectories: true } })).toBe(true)
    expect(readsProject({ spec: tree, caps: { load: true } })).toBe(false)
    expect(readsProject({ spec: tree, caps: null })).toBe(false)
    expect(readsProject({ spec: { folder: {} }, caps: { additionalDirectories: true } })).toBe(false)
    expect(readsProject({ spec: null, caps: { additionalDirectories: true } })).toBe(false)
  })
})

describe('deleteQuestion', () => {
  it('says the agent deletes its copy only when it can', () => {
    expect(deleteQuestion({ caps: { delete: true } }, 'Grok')).toBe('Delete this chat? Mooring removes it and asks Grok to delete its own copy.')
    expect(deleteQuestion({ caps: { delete: false } }, 'Grok')).toBe('Delete this chat?')
    expect(deleteQuestion({ caps: null }, 'Grok')).toBe('Delete this chat?')
    expect(deleteQuestion(undefined, 'Grok')).toBe('Delete this chat?')
    expect(deleteQuestion({ caps: { delete: true }, spec: { agentMade: true } }, 'Grok')).toBe('Delete this chat?')
  })
})
