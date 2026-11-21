import { describe, expect, it } from 'vitest'
import { checkSession, login, loginWithPassword, logout, passwordIsSet, ServerError, type Fetcher } from './api'

type Call = { input: string; init?: RequestInit }

function fake(status: number, headers: Record<string, string> = {}) {
  const calls: Call[] = []
  const fetcher: Fetcher = async (input, init) => {
    calls.push({ input, init })
    return new Response(null, { status, headers })
  }
  return { fetcher, calls }
}

describe('checkSession', () => {
  it('maps 204 and 401 to signed-in and signed-out', async () => {
    expect(await checkSession(fake(204).fetcher)).toBe('signed-in')
    expect(await checkSession(fake(401).fetcher)).toBe('signed-out')
  })

  it('throws on any other answer', async () => {
    await expect(checkSession(fake(500).fetcher)).rejects.toBeInstanceOf(ServerError)
  })
})

describe('login', () => {
  it('posts the trimmed token as JSON', async () => {
    const { fetcher, calls } = fake(204)
    expect(await login('  abc \n', fetcher)).toEqual({ ok: true })
    expect(calls[0].input).toBe('/api/login')
    expect(calls[0].init?.method).toBe('POST')
    expect(calls[0].init?.body).toBe('{"token":"abc"}')
  })

  it('reports a wrong token', async () => {
    expect(await login('x', fake(401).fetcher)).toEqual({ ok: false, reason: 'wrong-token' })
  })

  it('reports the wait after too many attempts', async () => {
    const result = await login('x', fake(429, { 'Retry-After': '42' }).fetcher)
    expect(result).toEqual({ ok: false, reason: 'too-many-attempts', retryAfterSeconds: 42 })
  })

  it('falls back to a minute when Retry-After is missing', async () => {
    const result = await login('x', fake(429).fetcher)
    expect(result).toEqual({ ok: false, reason: 'too-many-attempts', retryAfterSeconds: 60 })
  })
})

describe('logout', () => {
  it('throws when the server refuses', async () => {
    await expect(logout(fake(403).fetcher)).rejects.toBeInstanceOf(ServerError)
    await expect(logout(fake(204).fetcher)).resolves.toBeUndefined()
  })
})

describe('loginWithPassword', () => {
  it('posts the trimmed username and the password as typed', async () => {
    const { fetcher, calls } = fake(204)
    expect(await loginWithPassword(' ada ', ' pass word ', fetcher)).toEqual({ ok: true })
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({ username: 'ada', password: ' pass word ' })
  })
  it('maps 401 to wrong-credentials and 429 to the wait', async () => {
    expect(await loginWithPassword('a', 'b', fake(401).fetcher)).toEqual({ ok: false, reason: 'wrong-credentials' })
    expect(await loginWithPassword('a', 'b', fake(429, { 'Retry-After': '12' }).fetcher)).toEqual({ ok: false, reason: 'too-many-attempts', retryAfterSeconds: 12 })
  })
})

describe('loginWithPassword and an unusable password file', () => {
  it('tells it apart from a wrong password', async () => {
    const f: Fetcher = async () => new Response(JSON.stringify({ error: 'password_file_unreadable' }), { status: 401 })
    expect(await loginWithPassword('a', 'b', f)).toEqual({ ok: false, reason: 'password-file' })
  })
})

describe('passwordIsSet', () => {
  it('reads passwordSet', async () => {
    const f: Fetcher = async () => new Response(JSON.stringify({ passwordSet: true }))
    expect(await passwordIsSet(f)).toBe(true)
  })
  it('throws on a server error', async () => {
    await expect(passwordIsSet(fake(500).fetcher)).rejects.toBeInstanceOf(ServerError)
  })
})
