import { describe, expect, it } from 'vitest'
import { chordOf, showKey } from './keys'

const k = (code: string, o: Partial<KeyboardEvent> = {}) => ({ code, metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, ...o })

describe('showKey', () => {
  it('uses Mac symbols on a Mac only', () => {
    expect(showKey('Alt+G', true)).toBe('⌥G')
    expect(showKey('Alt+Shift+Right', true)).toBe('⌥⇧→')
    expect(showKey('Alt+Esc', true)).toBe('⌥Esc')
    expect(showKey('Alt+G', false)).toBe('Alt+G')
  })
})

describe('chordOf', () => {
  it('reads Cmd chords on a Mac', () => {
    expect(chordOf(k('KeyP', { metaKey: true }), false, true)).toBe('file')
    expect(chordOf(k('KeyP', { metaKey: true, shiftKey: true }), false, true)).toBe('palette')
    expect(chordOf(k('KeyF', { metaKey: true, shiftKey: true }), false, true)).toBe('search')
    expect(chordOf(k('Comma', { metaKey: true }), false, true)).toBe('settings')
  })
  it('leaves Cmd+W and Cmd+T to the browser, and Ctrl alone on a Mac', () => {
    expect(chordOf(k('KeyW', { metaKey: true }), false, true)).toBeNull()
    expect(chordOf(k('KeyT', { metaKey: true }), false, true)).toBeNull()
    expect(chordOf(k('KeyP', { ctrlKey: true }), false, true)).toBeNull()
    expect(chordOf(k('KeyP', { metaKey: true, altKey: true }), false, true)).toBeNull()
  })
  it('keeps Ctrl+P for a shell outside a Mac', () => {
    expect(chordOf(k('KeyP', { ctrlKey: true }), true, false)).toBeNull()
    expect(chordOf(k('KeyP', { ctrlKey: true }), false, false)).toBe('file')
    expect(chordOf(k('KeyP', { ctrlKey: true, shiftKey: true }), true, false)).toBe('palette')
    expect(chordOf(k('KeyP', { ctrlKey: true }), true, true)).toBeNull()
  })
})

describe('input method composition', () => {
  const base = { code: 'KeyP', metaKey: true, ctrlKey: false, altKey: false, shiftKey: false }
  it('is never a chord', () => {
    expect(chordOf(base, false, true)).toBe('file')
    expect(chordOf({ ...base, isComposing: true }, false, true)).toBeNull()
    expect(chordOf({ ...base, keyCode: 229 }, false, true)).toBeNull()
  })
})
