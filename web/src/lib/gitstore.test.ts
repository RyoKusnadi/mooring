import { afterEach, describe, expect, it, vi } from 'vitest'
import { git, POLL_MS } from './gitstore.svelte'

const status = (branch: string) => ({ repo: true, branch, detached: false, head: 'h', upstream: '', ahead: 0, behind: 0, files: [], stagedOutside: 0, truncated: false, conflicted: false, pushed: false })

function serve(answer: (url: string) => unknown) {
  const urls: string[] = []
  vi.stubGlobal('fetch', async (u: string) => (urls.push(u), new Response(JSON.stringify(answer(u)))))
  return urls
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
  git.select('')
})

describe('git store', () => {
  it('keeps an unsent message per place', () => {
    serve(() => ({ worktrees: [] }))
    git.select('p1')
    git.message = 'fix login'
    git.setFolder('.mooring/worktrees/b')
    expect(git.message).toBe('')
    git.message = 'other'
    git.setFolder('')
    expect(git.message).toBe('fix login')
    git.select('p2')
    expect(git.message).toBe('')
    git.select('p1')
    expect(git.message).toBe('fix login')
  })

  it('does not clear text typed while a commit runs', async () => {
    serve(() => status('main'))
    git.select('p3')
    let finish: (v: unknown) => void = () => {}
    git.attach(<T>() => new Promise<T>((r) => (finish = r as (v: unknown) => void)))
    git.message = 'first'
    const done = git.commit()
    git.message = 'second, typed meanwhile'
    finish({ commit: 'abc', branch: 'main' })
    expect(await done).toBe(true)
    expect(git.message).toBe('second, typed meanwhile')
    expect(git.notice).toContain('abc')
  })

  it('drops a status read for the project left behind', async () => {
    let release: () => void = () => {}
    const gate = new Promise<void>((r) => (release = r))
    vi.stubGlobal('fetch', async (u: string) => {
      if (u.includes('status')) await gate
      return new Response(JSON.stringify(u.includes('status') ? status('old') : { worktrees: [] }))
    })
    git.select('p4')
    git.select('')
    release()
    await new Promise((r) => setTimeout(r, 10))
    expect(git.status).toBeNull()
  })

  it('reads every few seconds while held, quietly, and stops when released', async () => {
    vi.useFakeTimers()
    const urls = serve((u) => (u.includes('status') ? status('main') : { worktrees: [] }))
    git.select('p5')
    await vi.advanceTimersByTimeAsync(0)
    const reads = () => urls.filter((u) => u.includes('status')).length
    const before = reads()
    const release = git.hold()
    await vi.advanceTimersByTimeAsync(POLL_MS)
    expect(reads()).toBe(before + 1)
    expect(git.loading).toBe(false)
    release()
    await vi.advanceTimersByTimeAsync(POLL_MS * 3)
    expect(reads()).toBe(before + 1)
  })
})
