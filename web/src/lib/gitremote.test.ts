import { afterEach, describe, expect, it, vi } from 'vitest'
import { CommandError } from './events'
import type { Status } from './gitapi'
import { amendBlocked, fetchNotice, fetchRemote, headPushed, offersPull, pullLabel, pullNotice, pullRemote } from './gitremote.svelte'
import { git } from './gitstore.svelte'

const status = (o: Partial<Status>): Status => ({ repo: true, branch: 'main', detached: false, head: 'abc1234', upstream: 'origin/main', ahead: 0, behind: 0, files: [], stagedOutside: 0, truncated: false, conflicted: false, pushed: true, ...o })

afterEach(() => {
  vi.unstubAllGlobals()
  git.select('')
})

describe('git remote', () => {
  it('words fetch and pull results', () => {
    expect(fetchNotice('origin/main', 2)).toBe('Fetched. origin/main has 2 new commits.')
    expect(fetchNotice('origin/main', 1)).toBe('Fetched. origin/main has 1 new commit.')
    expect(fetchNotice('origin/main', 0)).toBe('Already up to date.')
    expect(pullNotice('origin/main', 3)).toBe('Pulled 3 commits from origin/main.')
    expect(pullNotice('origin/main', 0)).toBe('Already up to date.')
  })

  it('offers Pull only for a rejected push', () => {
    expect(offersPull({ code: 'PUSH_REJECTED' })).toBe(true)
    expect(offersPull({ code: 'AUTH_FAILED' })).toBe(false)
    expect(offersPull(null)).toBe(false)
  })

  it('labels the pull button by how far behind the branch is', () => {
    expect(pullLabel(status({ behind: 0 }))).toBe('')
    expect(pullLabel(status({ behind: 1 }))).toBe('Pull 1 commit')
    expect(pullLabel(status({ behind: 4 }))).toBe('Pull 4 commits')
  })

  it('blocks amend once the upstream has the last commit', () => {
    expect(headPushed(status({}))).toBe(true)
    expect(headPushed(status({ ahead: 1, pushed: false }))).toBe(false)
    expect(headPushed(status({ upstream: '', pushed: true }))).toBe(true)
    expect(headPushed(status({ detached: true, upstream: '', pushed: true }))).toBe(true)
    expect(headPushed(status({ upstream: '', pushed: false }))).toBe(false)
    expect(headPushed(status({ head: '' }))).toBe(false)
    expect(amendBlocked(status({ upstream: '' }))).toContain('already on a remote branch')
    expect(amendBlocked(status({}))).toBe('The last commit is already on origin/main. Changing it would need a force push, which Mooring does not do.')
  })

  it('keeps a rejected push as a failure and pulls from it', async () => {
    vi.stubGlobal('fetch', async () => new Response(JSON.stringify({ worktrees: [] })))
    git.select('p9')
    const calls: string[] = []
    git.attach(<T>(name: string) => {
      calls.push(name)
      if (name === 'git.push') return Promise.reject(new CommandError('PUSH_REJECTED', 'The remote has commits you do not have. Pull first.'))
      return Promise.resolve({ upstream: 'origin/main', pulled: 1 } as T)
    })
    expect(await git.push(false)).toBe(false)
    expect(offersPull(git.failure)).toBe(true)
    expect(git.failure?.message).toBe('The remote has commits you do not have. Pull first.')
    expect(await pullRemote()).toBe(true)
    expect(git.failure).toBeNull()
    expect(git.notice).toBe('Pulled 1 commit from origin/main.')
    expect(calls).toEqual(['git.push', 'git.pull'])
  })

  it('shows the fetch notice and sends no amend flag by default', async () => {
    vi.stubGlobal('fetch', async () => new Response(JSON.stringify({ worktrees: [] })))
    git.select('p8')
    const sent: unknown[] = []
    git.attach(<T>(name: string, args: unknown) => {
      sent.push([name, args])
      return Promise.resolve({ upstream: 'origin/main', new: 0, commit: 'c', branch: 'main' } as T)
    })
    await fetchRemote()
    expect(git.notice).toBe('Already up to date.')
    git.message = 'x'
    await git.commit()
    git.amend = true
    git.message = 'y'
    await git.commit()
    expect(sent[1]).toEqual(['git.commit', { projectId: 'p8', folder: '', message: 'x' }])
    expect(sent[2]).toEqual(['git.commit', { projectId: 'p8', folder: '', message: 'y', amend: true }])
    expect(git.amend).toBe(false)
  })
})
