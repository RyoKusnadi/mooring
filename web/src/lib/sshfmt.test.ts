import { describe, expect, it } from 'vitest'
import { refusal, draftOfConfig, emptyDraft, forwardProblem, hostLabel, hostProblem, readKeyChange } from './sshfmt'

const good = { ...emptyDraft(), name: 'Box', address: 'box.example.com', user: 'me' }

describe('hostProblem', () => {
  it('accepts a complete host', () => expect(hostProblem(good)).toBe(''))
  it('asks for each missing part by name', () => {
    expect(hostProblem({ ...good, name: ' ' })).toMatch(/name/)
    expect(hostProblem({ ...good, address: '' })).toMatch(/address/)
    expect(hostProblem({ ...good, port: NaN })).toMatch(/port/)
    expect(hostProblem({ ...good, user: '' })).toMatch(/user/)
    expect(hostProblem({ ...good, auth: 'key' })).toMatch(/key file/)
    expect(hostProblem({ ...good, root: 'work' })).toMatch(/absolute/)
  })
})

describe('forwardProblem', () => {
  it('allows local port 0', () => expect(forwardProblem(0, 'localhost', 80)).toBe(''))
  it('rejects bad ports and an empty host', () => {
    expect(forwardProblem(70000, 'localhost', 80)).toMatch(/local port/)
    expect(forwardProblem(0, ' ', 80)).toMatch(/host/)
    expect(forwardProblem(0, 'localhost', NaN)).toMatch(/remote port/)
  })
})

describe('readKeyChange', () => {
  const base = 'The host key of [h]:22 changed. It was SHA256:aaa+/ and is now SHA256:bbb. Someone may be in the middle, or the server was reinstalled. '
  it('reads both fingerprints and the advice', () => {
    const k = readKeyChange(base + 'The old key is in /data/known_hosts; if the change is expected, forget the host key first.')
    expect(k).toMatchObject({ was: 'SHA256:aaa+/', now: 'SHA256:bbb' })
    expect(k.advice).toMatch(/^The old key is in \/data\/known_hosts/)
  })
  it('falls back to the whole text', () => expect(readKeyChange('odd').advice).toBe('odd'))
})

describe('labels', () => {
  it('names a host as user@address:port', () => expect(hostLabel({ user: 'me', address: 'a.b', port: 2222 })).toBe('me@a.b:2222'))
  it('imports a config entry with its key file', () => {
    const d = draftOfConfig({ name: 'x', address: 'a', port: 0, user: 'u', keyPath: '~/.ssh/k', agentForward: false })
    expect(d).toMatchObject({ port: 22, auth: 'key', keyPath: '~/.ssh/k' })
  })
})

describe('refusal', () => {
  it('speaks only for a retried password or passphrase', () => {
    expect(refusal({ kind: 'password' })).toBe('')
    expect(refusal({ kind: 'password', retry: true })).toBe('That password was refused. Try again.')
    expect(refusal({ kind: 'passphrase', retry: true })).toMatch(/did not unlock the key/)
    expect(refusal({ kind: 'question', retry: true })).toBe('')
  })
})
