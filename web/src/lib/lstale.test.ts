import { describe, expect, it } from 'vitest'
import { ignoreLateCompletions, isLateFieldError } from './lstale'

describe('late completion errors', () => {
  it('knows the field error and nothing else', () => {
    expect(isLateFieldError(new RangeError('Field is not present in this state'))).toBe(true)
    expect(isLateFieldError(new RangeError('Invalid position 5'))).toBe(false)
    expect(isLateFieldError(new Error('Field is not present in this state'))).toBe(false)
    expect(isLateFieldError('Field is not present in this state')).toBe(false)
  })

  it('stops only that error from being reported', () => {
    let handler: (e: unknown) => void = () => {}
    ignoreLateCompletions({ addEventListener: (_t: string, h: unknown) => (handler = h as typeof handler) } as unknown as Window)
    const calls: string[] = []
    const event = (error: unknown) => ({ error, preventDefault: () => calls.push('prevented') })
    handler(event(new RangeError('Field is not present in this state')))
    handler(event(new Error('other')))
    expect(calls).toEqual(['prevented'])
  })
})
