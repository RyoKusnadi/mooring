import { describe, expect, it, vi } from 'vitest'
import { bellCounts, makeNotifier, notifyText, sendTest, setBellCounts, TEST_TEXT, type Needing } from './notify'

const term = (key: string, project = 'Alpha'): Needing => ({ key, kind: 'terminal', project })

function setup(hidden = true, allowed = true, prime = true) {
  const state = { hidden, allowed }
  const show = vi.fn()
  const sync = makeNotifier({ hidden: () => state.hidden, allowed: () => state.allowed, show })
  if (prime) sync([])
  return { state, show, sync }
}

describe('notifier', () => {
  it('fires once per rise, with one tag per item', () => {
    const { show, sync } = setup()
    sync([term('t1')])
    sync([term('t1')])
    expect(show).toHaveBeenCalledTimes(1)
    expect(show).toHaveBeenCalledWith('A terminal in Alpha needs you.', 'mooring-t1')
    sync([])
    sync([term('t1')])
    expect(show).toHaveBeenCalledTimes(2)
    sync([term('t1'), { key: 'c1', kind: 'chat', project: 'Beta' }])
    expect(show).toHaveBeenLastCalledWith('A chat in Beta needs you.', 'mooring-c1')
  })

  it('does not announce what already needed you when the page loaded', () => {
    const { show, sync } = setup(true, true, false)
    sync([], false)
    sync([term('t1')], false)
    sync([term('t1')])
    sync([term('t1'), term('t2')])
    expect(show).toHaveBeenCalledTimes(1)
    expect(show).toHaveBeenCalledWith('A terminal in Alpha needs you.', 'mooring-t2')
  })

  it('stays quiet while the tab is visible, and does not announce that item later', () => {
    const { state, show, sync } = setup(false)
    sync([term('t1')])
    state.hidden = true
    sync([term('t1')])
    expect(show).not.toHaveBeenCalled()
  })

  it('stays quiet without permission', () => {
    const { show, sync } = setup(true, false)
    sync([term('t1')])
    expect(show).not.toHaveBeenCalled()
  })

  it('never carries anything but the project', () => {
    expect(notifyText({ kind: 'terminal', project: '' })).toBe('A terminal in a project needs you.')
  })
})

describe('bell switch', () => {
  it('is on until turned off', () => {
    const data = new Map<string, string>()
    const store = { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v) }
    expect(bellCounts(store)).toBe(true)
    setBellCounts(false, store)
    expect(bellCounts(store)).toBe(false)
    setBellCounts(true, store)
    expect(bellCounts(store)).toBe(true)
  })
})

describe('sendTest', () => {
  it('shows one notification with the test text when allowed', () => {
    const make = vi.fn()
    expect(sendTest({ allowed: () => true, make })).toBe(true)
    expect(make).toHaveBeenCalledWith('Mooring', { body: TEST_TEXT, tag: 'mooring-test' })
  })

  it('shows nothing when notifications are not allowed', () => {
    const make = vi.fn()
    expect(sendTest({ allowed: () => false, make })).toBe(false)
    expect(make).not.toHaveBeenCalled()
  })

  it('answers false when the browser refuses to make one', () => {
    const make = vi.fn(() => {
      throw new TypeError('Illegal constructor')
    })
    expect(sendTest({ allowed: () => true, make })).toBe(false)
  })
})
