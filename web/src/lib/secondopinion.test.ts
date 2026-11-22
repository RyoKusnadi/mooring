import { describe, expect, it } from 'vitest'
import { otherAgent } from './secondopinion'

const a = (id: string, installed = true) => ({ id, installed })

describe('otherAgent', () => {
  it('skips the earlier chat agent', () => {
    expect(otherAgent([a('x'), a('y')], 'x')?.id).toBe('y')
  })
  it('skips agents that are not installed', () => {
    expect(otherAgent([a('x'), a('y', false), a('z')], 'x')?.id).toBe('z')
  })
  it('is undefined when only the same agent is installed', () => {
    expect(otherAgent([a('x'), a('y', false)], 'x')).toBeUndefined()
  })
})
