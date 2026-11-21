import { describe, expect, it } from 'vitest'
import { answerText, attentionLabel, canSummarize, projectAttentionLabel, waitingText } from './chatfmt'

const perm = { attention: true }
const limit = { attention: true, limitOnly: true }
const quiet = { attention: false }

describe('attention labels', () => {
  it('names a limit apart from a permission', () => {
    expect(attentionLabel(perm)).toBe('Needs permission')
    expect(attentionLabel(limit)).toBe('Usage limit reached')
  })
  it('lets a waiting permission win on a project row', () => {
    expect(projectAttentionLabel([limit, perm])).toBe('Needs permission')
    expect(projectAttentionLabel([limit, quiet])).toBe('Usage limit reached')
  })
  it('counts waiting chats in plain words', () => {
    expect(waitingText([perm, quiet])).toBe('1 chat needs permission')
    expect(waitingText([perm, perm])).toBe('2 chats need permission')
    expect(waitingText([limit])).toBe('1 chat stopped at a usage limit')
    expect(waitingText([limit, limit])).toBe('2 chats stopped at a usage limit')
    expect(waitingText([limit, perm])).toBe('2 chats need you')
  })
})

describe('canSummarize', () => {
  it('allows idle, stopped and signed-out chats with messages', () => {
    for (const state of ['idle', 'stopped', 'needs_login']) expect(canSummarize({ state, messages: 2 })).toBe(true)
  })
  it('refuses an empty chat and one that is starting or replying', () => {
    expect(canSummarize({ state: 'idle', messages: 0 })).toBe(false)
    for (const state of ['starting', 'running']) expect(canSummarize({ state, messages: 2 })).toBe(false)
  })
})

describe('answerText', () => {
  it('says what the user did', () => {
    expect(answerText('answered', { name: 'Allow', kind: 'allow_once' })).toBe('You allowed')
    expect(answerText('answered', { name: 'Allow always', kind: 'allow_always' })).toBe('You allowed always')
    expect(answerText('answered', { name: 'Reject', kind: 'reject_once' })).toBe('You refused')
    expect(answerText('cancelled', undefined)).toBe('Cancelled')
  })
})

describe('composerWhy', () => {
  it('says why a chat cannot take a message, and nothing when it can', async () => {
    const { composerWhy } = await import('./chatfmt')
    expect(composerWhy('starting')).toMatch(/starting/)
    expect(composerWhy('needs_login')).toMatch(/Sign in/)
    expect(composerWhy('stopped')).toMatch(/stopped/)
    expect(composerWhy('idle')).toBe('')
  })
})
