import { describe, expect, it } from 'vitest'
import { CommandError } from './events'
import { FileError } from './filesapi'
import { getDiff, getStatus, getWorktrees, gitCanRetry, gitError, pathsOf, sideLetter, sideWord, stagedOf, unstagedOf, type Change, type Status } from './gitapi'

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status })
const change = (c: Partial<Change>): Change => ({ path: 'a', staged: '', unstaged: '', kind: '', ...c })

describe('gitapi reads', () => {
  it('encodes the place and the path', async () => {
    const urls: string[] = []
    const f = async (u: string) => (urls.push(u), json({ repo: true }))
    await getStatus('p1', '.mooring/worktrees/ab', f)
    await getDiff('p1', '', 'dir/a b.txt', true, f)
    expect(urls).toEqual(['/api/git/status?project=p1&folder=.mooring%2Fworktrees%2Fab', '/api/git/diff?project=p1&folder=&path=dir%2Fa%20b.txt&staged=1'])
  })

  it('turns an error body into a FileError and unwraps worktrees', async () => {
    const err = await getStatus('p', '', async () => json({ error: { code: 'OUTSIDE_PROJECT', message: 'No.' } }, 403)).catch((e) => e)
    expect(err).toBeInstanceOf(FileError)
    expect(err.code).toBe('OUTSIDE_PROJECT')
    expect(await getWorktrees('p', async () => json({ worktrees: [{ folder: 'f', branch: 'b', head: 'h' }] }))).toHaveLength(1)
  })
})

describe('lists and words', () => {
  const s = {
    files: [
      change({ path: 'new.txt', unstaged: '', kind: 'untracked' }),
      change({ path: 'gone.txt', unstaged: 'deleted' }),
      change({ path: 'b.txt', origPath: 'a.txt', staged: 'renamed' }),
      change({ path: 'both.txt', staged: 'modified', unstaged: 'modified' }),
    ],
  } as Status

  it('puts a file with both sides in both lists', () => {
    expect(stagedOf(s).map((c) => c.path)).toEqual(['b.txt', 'both.txt'])
    expect(unstagedOf(s).map((c) => c.path)).toEqual(['new.txt', 'gone.txt', 'both.txt'])
  })

  it('names new, deleted and renamed files in words', () => {
    expect(sideWord(s.files[0], false)).toBe('new')
    expect(sideWord(s.files[1], false)).toBe('deleted')
    expect(sideWord(s.files[2], true)).toBe('renamed')
    expect(sideLetter(s.files[0], false)).toBe('?')
    expect(sideLetter(s.files[1], false)).toBe('D')
    expect(sideLetter(s.files[2], true)).toBe('R')
    expect(sideWord(change({ kind: 'conflicted' }), false)).toBe('conflict')
  })

  it('names both paths of a staged rename', () => {
    expect(pathsOf(s.files[2], true)).toEqual(['b.txt', 'a.txt'])
    expect(pathsOf(s.files[0], false)).toEqual(['new.txt'])
  })
})

describe('gitError', () => {
  it('has plain words for the contract codes', () => {
    expect(gitError(new CommandError('NO_IDENTITY', 'x')).message).toMatch(/user\.name/)
    expect(gitError(new CommandError('PUSH_REJECTED', 'x')).message).toMatch(/remote has commits/)
    expect(gitError(new CommandError('DIRTY', 'x')).message).toMatch(/uncommitted/)
    expect(gitError(new CommandError('BUSY', '')).message).toMatch(/chat/)
    expect(gitError(new CommandError('BUSY', 'A terminal is open in this worktree. Close it first.')).message).toMatch(/^A terminal is open/)
  })

  it('shows hook output and git\'s own message', () => {
    expect(gitError(new CommandError('HOOK_FAILED', 'lint failed: 2 errors')).message).toBe('A commit hook refused the commit.\nlint failed: 2 errors')
    expect(gitError(new CommandError('GIT_FAILED', 'fatal: weird')).message).toBe('fatal: weird')
  })

  it('tells a dropped connection from a refusal', () => {
    expect(gitError(new TypeError('Failed to fetch')).message).toMatch(/Could not reach/)
    expect(gitError(new Error('x')).message).toBe('Git could not do that. Try again.')
  })
})

describe('gitCanRetry', () => {
  it('offers Try again only for causes that can pass', () => {
    expect(gitCanRetry('NETWORK')).toBe(true)
    expect(gitCanRetry('BUSY')).toBe(true)
    expect(gitCanRetry('NOT_A_REPO')).toBe(false)
    expect(gitCanRetry('NOT_FOUND')).toBe(false)
  })
})
