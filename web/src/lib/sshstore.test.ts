import { describe, expect, it } from 'vitest'
import { CommandError } from './events'
import { ssh } from './sshstore.svelte'
import type { SshPrompt } from './sshapi'

const prompt = { promptId: 'p1', kind: 'password', host: 'e2e@h:22' } as SshPrompt

describe('ssh.answer', () => {
  it('keeps the question open when the answer fails, and closes it when it works', async () => {
    ssh.prompts = [prompt]
    ssh.attach(() => Promise.reject(new CommandError('INTERNAL', 'no')))
    await expect(ssh.answer('p1', { value: 'x' })).rejects.toThrow('no')
    expect(ssh.prompts).toHaveLength(1)
    ssh.attach(() => Promise.resolve({} as never))
    await ssh.answer('p1', { value: 'x' })
    expect(ssh.prompts).toHaveLength(0)
  })

  it('treats an answer given elsewhere as done', async () => {
    ssh.prompts = [prompt]
    ssh.attach(() => Promise.reject(new CommandError('NOT_FOUND', 'gone')))
    await ssh.answer('p1', {})
    expect(ssh.prompts).toHaveLength(0)
  })
})

describe('ssh.connect', () => {
  it('sends a new connect after a failed one', async () => {
    let calls = 0
    ssh.attach((name) => {
      if (name === 'ssh.connect') calls++
      return calls === 1 ? Promise.reject(new CommandError('HOST_KEY_CHANGED', 'x')) : Promise.resolve({ id: 'c1', hostId: 'h', state: 'connected' } as never)
    })
    await expect(ssh.connect('h')).rejects.toThrow()
    ssh.failure = null
    await expect(ssh.connect('h')).resolves.toBe('c1')
    expect(calls).toBe(2)
  })
})

describe('ssh.arm', () => {
  it('drops the typed password when the connect ends before any question', async () => {
    const answers: string[] = []
    let connId = ''
    ssh.attach((name, args) => {
      if (name === 'ssh.answer') answers.push(name)
      if (name === 'ssh.connect') {
        connId = (args as { id: string }).id
        return Promise.reject(new CommandError('INTERNAL', 'down'))
      }
      return Promise.resolve({} as never)
    })
    const p = ssh.connect('h2', true)
    ssh.arm(connId, { value: 'secret', remember: false })
    await expect(p).rejects.toThrow()
    ssh.apply('ssh.prompt', { promptId: 'q', connId, kind: 'password' })
    expect(answers).toEqual([])
    expect(ssh.prompts.map((x) => x.promptId)).toContain('q')
  })
})

describe('ssh.saveHost', () => {
  const draft = { name: 'n', address: 'new.example', port: 22, user: 'u', auth: 'password' as const, keyPath: '', agentForward: false, root: '', id: 'h3' }
  it('closes the live connection of a host whose address changed, so the test uses the new one', async () => {
    const calls: string[] = []
    ssh.hosts = [{ ...draft, address: 'old.example', remembered: false, savedKey: '' }]
    ssh.conns = [{ id: 'c9', hostId: 'h3', state: 'connected' } as never]
    ssh.attach((name) => {
      calls.push(name)
      return Promise.resolve(name === 'ssh.hostSave' ? ({ ...draft, remembered: false, savedKey: '' } as never) : ({} as never))
    })
    await ssh.saveHost(draft)
    expect(calls).toEqual(['ssh.disconnect', 'ssh.hostSave'])
  })
  it('keeps it when only the name changed', async () => {
    const calls: string[] = []
    ssh.hosts = [{ ...draft, name: 'other', remembered: false, savedKey: '' }]
    ssh.conns = [{ id: 'c9', hostId: 'h3', state: 'connected' } as never]
    ssh.attach((name) => {
      calls.push(name)
      return Promise.resolve({ ...draft, remembered: false, savedKey: '' } as never)
    })
    await ssh.saveHost(draft)
    expect(calls).toEqual(['ssh.hostSave'])
  })
})
