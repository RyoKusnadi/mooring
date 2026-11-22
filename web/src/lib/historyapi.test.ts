import { describe, expect, it } from 'vitest'
import { getLog, sameWho, relativeTime, shortId } from './historyapi'
import type { Fetcher } from './api'

const NOW = 1_800_000_000_000

describe('relativeTime', () => {
  const ago = (s: number) => relativeTime(NOW / 1000 - s, NOW)
  it('names the largest unit', () => {
    expect(ago(10)).toBe('just now')
    expect(ago(60)).toBe('1 minute ago')
    expect(ago(60 * 5)).toBe('5 minutes ago')
    expect(ago(3600 * 3)).toBe('3 hours ago')
    expect(ago(86400)).toBe('1 day ago')
    expect(ago(86400 * 8)).toBe('1 week ago')
    expect(ago(86400 * 70)).toBe('2 months ago')
    expect(ago(86400 * 400)).toBe('1 year ago')
    expect(ago(86400 * 365 * 4)).toBe('4 years ago')
  })
  it('a commit dated in the future reads as just now', () => expect(relativeTime(NOW / 1000 + 500, NOW)).toBe('just now'))
})

describe('getLog', () => {
  it('sends the cursor and the search only when set', async () => {
    const urls: string[] = []
    const f: Fetcher = async (u) => (urls.push(String(u)), new Response('{"commits":[],"next":"","total":0}'))
    await getLog('p1', '', { limit: 200 }, undefined, f)
    await getLog('p1', 'wt', { cursor: 'c2', q: 'fix bug', in: 'code' }, undefined, f)
    expect(urls[0]).toBe('/api/git/log?project=p1&folder=&limit=200')
    expect(urls[1]).toBe('/api/git/log?project=p1&folder=wt&cursor=c2&q=fix%20bug&in=code')
  })
  it('short ids are seven characters', () => expect(shortId('0123456789abcdef')).toBe('0123456'))
})

describe('sameWho', () => {
  it('compares name and email, not the written form', () => {
    expect(sameWho('Ann Dev <A@b.c>', 'Ann Dev', 'a@b.c')).toBe(true)
    expect(sameWho('Ann Dev', 'Ann Dev', 'a@b.c')).toBe(true)
    expect(sameWho('GitHub <noreply@github.com>', 'Ann Dev', 'a@b.c')).toBe(false)
  })
})
