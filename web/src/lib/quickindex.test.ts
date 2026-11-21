import { describe, expect, it } from 'vitest'
import { cached, forgetIndex, remember } from './quickindex'

describe('index cache', () => {
  it('keeps an index for a short while per project', () => {
    forgetIndex('p')
    expect(cached('p')).toBeNull()
    remember('p', { paths: ['a'], complete: true }, 1000)
    expect(cached('p', 1000 + 29_000)?.paths).toEqual(['a'])
    expect(cached('p', 1000 + 31_000)).toBeNull()
  })
})
