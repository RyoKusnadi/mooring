import { describe, expect, it } from 'vitest'
import { endText } from './termend'

describe('endText', () => {
  it('names a lost SSH connection without its code', () => {
    expect(endText({ ssh: 'c1', code: 255, lost: true })).toBe('The connection to the host ended.')
  })
  it('gives the exit code of a shell, local or remote', () => {
    expect(endText({ code: 2 })).toBe('The shell exited with code 2.')
    expect(endText({ ssh: 'c1', code: 143 })).toBe('The shell exited with code 143.')
    expect(endText({ ssh: 'c1', code: 0 })).toBe('The shell exited.')
    expect(endText(undefined)).toBe('The shell exited.')
  })
})
