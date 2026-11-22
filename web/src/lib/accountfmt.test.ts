import { describe, expect, it } from 'vitest'
import { usageLine } from './accountfmt'

const base = { source: 'agent' as const, limitHit: 0, limitResetsAt: 0, updatedAt: 0 }
const now = new Date(2026, 0, 5, 10, 0).getTime()
const at = (h: number) => new Date(2026, 0, 5, h, 0).getTime()

describe('usageLine', () => {
  it('names the most used window and lists them all', () => {
    const u = { ...base, windows: [{ name: 'five_hour', usedPercent: 62, resetsAt: at(14), status: '' }, { name: 'seven_day', usedPercent: 10, resetsAt: at(23), status: '' }] }
    const l = usageLine(u, now)!
    expect(l.text).toMatch(/^5 hours: 62% used, resets at /)
    expect(l.all.split('\n')).toHaveLength(2)
    expect(l.all).toContain('7 days: 10% used')
  })
  it('shows nothing when unknown', () => {
    expect(usageLine({ ...base, source: 'unknown', windows: [] }, now)).toBeNull()
    expect(usageLine({ ...base, windows: [{ name: 'five_hour', usedPercent: null, resetsAt: 0, status: '' }] }, now)).toBeNull()
  })
  it('lists a window with no percentage as unknown, never a guess', () => {
    const u = { ...base, windows: [{ name: 'five_hour', usedPercent: 5, resetsAt: 0, status: '' }, { name: 'seven_day', usedPercent: null, resetsAt: 0, status: '' }] }
    expect(usageLine(u, now)!.all).toContain('7 days: unknown')
  })
  it('skips a window whose reset time has passed', () => {
    const u = { ...base, windows: [{ name: 'five_hour', usedPercent: 98, resetsAt: at(9), status: '' }, { name: 'seven_day', usedPercent: 10, resetsAt: at(23), status: '' }] }
    const l = usageLine(u, now)!
    expect(l.text).toMatch(/^7 days: 10% used/)
    expect(l.all).toMatch(/5 hours: not reported since the reset at /)
    expect(usageLine({ ...base, windows: [u.windows[0]] }, now)).toBeNull()
  })
})
