import { describe, expect, it } from 'vitest'
import { asksFirst, inline, loginCommand, prose, tokens } from './chatfmt'
import { buildOffsets, canScroll, followAfterScroll, rowAt, visibleRange } from './virtual'

describe('virtual list math', () => {
  const o = buildOffsets(4, [10, undefined, 30, 40], 20)
  it('sums measured and estimated heights', () => {
    expect([...o]).toEqual([0, 10, 30, 60, 100])
  })
  it('finds the row at a position, clamped', () => {
    expect(rowAt(o, 0)).toBe(0)
    expect(rowAt(o, 10)).toBe(1)
    expect(rowAt(o, 59)).toBe(2)
    expect(rowAt(o, 9999)).toBe(3)
  })
  it('adds overscan and stays inside the list', () => {
    expect(visibleRange(o, 35, 10, 1)).toEqual([1, 4])
    expect(visibleRange(buildOffsets(0, [], 20), 0, 100, 3)).toEqual([0, 0])
  })
  it('finds a row quickly in a long list', () => {
    const big = buildOffsets(2000, [], 50)
    expect(rowAt(big, 50 * 1500 + 1)).toBe(1500)
  })
})

describe('chat text helpers', () => {
  it('shortens token counts', () => {
    expect(tokens(900)).not.toBe('')
    expect(tokens(1200)).toMatch(/1\.2k/)
  })
  it('finds the login command in a note', () => {
    expect(loginCommand('Sign in with `gemini login` first.')).toBe('gemini login')
    expect(loginCommand(undefined)).toBe('')
    expect(loginCommand('Sign in with `a\nrm -rf x` first.')).toBe('a rm -rf x')
  })
})

describe('agent prose and modes', () => {
  it('keeps code fences and inline code, and closes an open fence', () => {
    expect(prose('Run `ls`.\n\n```sh\nls -la\n```\nDone')).toEqual([
      { kind: 'text', spans: [{ code: false, text: 'Run ' }, { code: true, text: 'ls' }, { code: false, text: '.' }] },
      { kind: 'code', text: 'ls -la' },
      { kind: 'text', spans: [{ code: false, text: 'Done' }] },
    ])
    expect(prose('```\nopen')).toEqual([{ kind: 'code', text: 'open' }])
    expect(inline('no code')).toEqual([{ code: false, text: 'no code' }])
  })
  it('confirms every mode that is not known to ask first', () => {
    const m = (id: string, name = id) => ({ id, name })
    for (const ok of [m('default', 'Manual'), m('plan', 'Plan'), m('ask')]) expect(asksFirst(ok)).toBe(true)
    for (const bad of [m('bypassPermissions'), m('acceptEdits'), m('dontAsk'), m('yolo'), m('auto', 'Auto'), m('whatever')]) expect(asksFirst(bad)).toBe(false)
    expect(asksFirst(undefined)).toBe(false)
  })
})

describe('jump to end', () => {
  it('shows only when the content is taller than the view', () => {
    expect(canScroll(300, 600)).toBe(false)
    expect(canScroll(620, 600)).toBe(false)
    expect(canScroll(900, 600)).toBe(true)
  })
})

describe('following the newest row', () => {
  const base = { follow: true, user: false, programmatic: false, prevTop: 500, top: 500, distance: 0 }
  it('stays on when content grows under it', () => {
    expect(followAfterScroll({ ...base, distance: 300 })).toBe(true)
  })
  it('stays on after the list moved the position itself', () => {
    expect(followAfterScroll({ ...base, programmatic: true, top: 100, distance: 900 })).toBe(true)
  })
  it('turns off when the user scrolls away', () => {
    expect(followAfterScroll({ ...base, user: true, top: 400, distance: 300 })).toBe(false)
  })
  it('turns off when the position moves up with no list scroll pending', () => {
    expect(followAfterScroll({ ...base, top: 400, distance: 300 })).toBe(false)
  })
  it('turns on again near the end, and stays off while away from it', () => {
    expect(followAfterScroll({ ...base, follow: false, user: true, distance: 10 })).toBe(true)
    expect(followAfterScroll({ ...base, follow: false, top: 600, distance: 300 })).toBe(false)
  })
})
