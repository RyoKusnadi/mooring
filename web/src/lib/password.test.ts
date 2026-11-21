import { describe, expect, it } from 'vitest'
import { checkNew, fetchPasswordStatus, signOutOthers } from './password'

describe('checkNew', () => {
  it('accepts a username and a matching password of 8 characters', () => {
    expect(checkNew(' ada ', 'abcdefgh', 'abcdefgh')).toBe('')
  })
  it('asks for a username first', () => {
    expect(checkNew('  ', 'abcdefgh', 'abcdefgh')).toBe('Enter a username.')
  })
  it('refuses a username over 64 characters', () => {
    expect(checkNew('a'.repeat(65), 'abcdefgh', 'abcdefgh')).toContain('at most 64')
    expect(checkNew('a'.repeat(64), 'abcdefgh', 'abcdefgh')).toBe('')
  })
  it('counts characters, not bytes', () => {
    expect(checkNew('ada', 'ééééééé', 'ééééééé')).toContain('at least 8')
    expect(checkNew('ada', 'éééééééé', 'éééééééé')).toBe('')
  })
  it('refuses two passwords that differ', () => {
    expect(checkNew('ada', 'abcdefgh', 'abcdefgx')).toBe('The two passwords are not the same.')
  })
})

describe('fetchPasswordStatus', () => {
  it('reads the status', async () => {
    const f = async () => new Response(JSON.stringify({ set: true, username: 'ada' }))
    expect(await fetchPasswordStatus(f)).toEqual({ set: true, username: 'ada' })
  })
})

describe('signOutOthers', () => {
  const answer = (status: number, headers: Record<string, string> = {}) => async () => new Response(null, { status, headers })
  it('answers an empty message on success', async () => {
    expect(await signOutOthers('pw', answer(204))).toBe('')
  })
  it('names a wrong password and the wait', async () => {
    expect(await signOutOthers('pw', answer(401))).toContain('not right')
    expect(await signOutOthers('pw', answer(429, { 'Retry-After': '9' }))).toContain('9 seconds')
    expect(await signOutOthers('pw', answer(500))).toContain('Try again')
  })
})
