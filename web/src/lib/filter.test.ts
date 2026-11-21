import { describe, expect, it } from 'vitest'
import { filterItems, looksLikeFile } from './filter'

// The palette ranks by match position, then keeps the list order, so list order is what "first" means.
describe('palette matching', () => {
  const items = [{ label: 'New terminal' }, { label: 'New chat' }, { label: 'Settings' }, { label: 'Switch to project new-site' }]
  it('keeps the given order for an empty query', () => {
    expect(filterItems(items, '').map((i) => i.label)).toEqual(items.map((i) => i.label))
  })
  it('puts a word-start hit before a later one', () => {
    expect(filterItems(items, 'new')[0].label).toBe('New terminal')
  })
})

describe('looksLikeFile', () => {
  it('accepts names with an extension or a slash', () => {
    for (const t of ['main.go', 'src/lib', 'App.svelte', 'a/b.ts']) expect(looksLikeFile(t), t).toBe(true)
  })
  it('rejects plain words and phrases', () => {
    for (const t of ['', 'terminal', 'new chat', 'split right', 'Show changes']) expect(looksLikeFile(t), t).toBe(false)
  })
})
