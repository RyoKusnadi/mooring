import { describe, expect, it } from 'vitest'
import { fitWidth, SIDEBAR_DEFAULT, SIDEBAR_MAX, SIDEBAR_MIN, clampWidth, readWidth, writeWidth } from './sidebarwidth'

const store = (init: Record<string, string> = {}) => {
  const m = { ...init }
  return { getItem: (k: string) => m[k] ?? null, setItem: (k: string, v: string) => void (m[k] = v) }
}

describe('sidebar width', () => {
  it('starts at 264 and stays between 220 and 480', () => {
    expect(SIDEBAR_DEFAULT).toBe(264)
    expect(clampWidth(100)).toBe(SIDEBAR_MIN)
    expect(clampWidth(900)).toBe(SIDEBAR_MAX)
    expect(clampWidth(300.4)).toBe(300)
    expect(clampWidth(NaN)).toBe(SIDEBAR_DEFAULT)
  })
  it('remembers the width and clamps what it reads back', () => {
    const s = store()
    expect(readWidth(s)).toBe(264)
    writeWidth(s, 340)
    expect(readWidth(s)).toBe(340)
    expect(readWidth(store({ 'mooring.sidebar': '5000' }))).toBe(480)
    expect(readWidth(store({ 'mooring.sidebar': 'x' }))).toBe(264)
  })
  it('survives storage that throws', () => {
    const broken = { getItem: () => { throw new Error('no') }, setItem: () => { throw new Error('no') } }
    expect(readWidth(broken)).toBe(264)
    expect(() => writeWidth(broken, 300)).not.toThrow()
    expect(readWidth(null)).toBe(264)
  })
})

it('fitWidth keeps 560px for the main column, but never goes under 220', () => {
  expect(fitWidth(480, 1440)).toBe(480)
  expect(fitWidth(480, 900)).toBe(340)
  expect(fitWidth(480, 700)).toBe(220)
})
