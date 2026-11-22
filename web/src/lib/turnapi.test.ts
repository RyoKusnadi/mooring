import { describe, expect, it } from 'vitest'
import { getLastTurn, getTurnDiff, kindWord, noTurnText } from './turnapi'

describe('turn api', () => {
  it('sends the project and the chat, and origPath only for a rename', async () => {
    const calls: [string, unknown][] = []
    const run = (async (n: string, a: unknown) => (calls.push([n, a]), {})) as never
    await getLastTurn(run, 'p', 'c')
    await getTurnDiff(run, 'p', 'c', 'b.txt')
    await getTurnDiff(run, 'p', 'c', 'new.txt', 'old.txt')
    expect(calls).toEqual([
      ['git.lastTurn', { projectId: 'p', chatId: 'c' }],
      ['git.turnDiff', { projectId: 'p', chatId: 'c', path: 'b.txt' }],
      ['git.turnDiff', { projectId: 'p', chatId: 'c', path: 'new.txt', origPath: 'old.txt' }],
    ])
  })
  it('names kinds and the empty state', () => {
    expect(kindWord('modified')).toBe('Changed')
    expect(noTurnText('Fix', false)).toBe('Fix has not finished a turn with a checkpoint yet.')
    expect(noTurnText('Fix', true)).toContain('Checkpoints are off')
  })
})
