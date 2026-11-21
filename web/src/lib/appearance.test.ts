import { describe, expect, it } from 'vitest'
import { oppositeTheme, readSize, readTheme, resolveTheme, write } from './appearance'

const store = (init: Record<string, string> = {}) => {
  const m = { ...init }
  return { getItem: (k: string) => m[k] ?? null, setItem: (k: string, v: string) => void (m[k] = v) }
}

describe('appearance', () => {
  it('defaults to system and the default size', () => {
    expect(readTheme(store())).toBe('system')
    expect(readSize(store())).toBe('default')
  })
  it('reads a stored choice and ignores junk', () => {
    expect(readTheme(store({ 'mooring.theme': 'dark' }))).toBe('dark')
    expect(readTheme(store({ 'mooring.theme': 'purple' }))).toBe('system')
    expect(readSize(store({ 'mooring.fontsize': 'large' }))).toBe('large')
  })
  it('survives storage that throws or is missing', () => {
    const broken = { getItem: () => { throw new Error('blocked') }, setItem: () => { throw new Error('blocked') } }
    expect(readTheme(broken)).toBe('system')
    expect(() => write('k', 'v', broken)).not.toThrow()
    expect(readTheme(null)).toBe('system')
  })
  it('system follows the browser, an explicit choice does not', () => {
    expect(resolveTheme('system', true)).toBe('dark')
    expect(resolveTheme('system', false)).toBe('light')
    expect(resolveTheme('light', true)).toBe('light')
    expect(resolveTheme('dark', false)).toBe('dark')
  })
  it('the quick switch flips what is drawn', () => {
    expect(oppositeTheme(resolveTheme('system', true))).toBe('light')
    expect(oppositeTheme('light')).toBe('dark')
  })
})
