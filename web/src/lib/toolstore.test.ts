import { beforeEach, describe, expect, it } from 'vitest'
import { CommandError } from './events'
import type { ToolServer } from './toolapi'
import { draftOfEntry, emptyDraft } from './toolfmt'
import { tools } from './toolstore.svelte'

const server = (o: Partial<ToolServer> = {}): ToolServer => ({
  id: 's1',
  name: 'Notes',
  kind: 'stdio',
  command: 'fake-mcp',
  args: [],
  env: [{ name: 'API_KEY', description: '', required: false, secret: true, set: true }],
  headers: [],
  url: '',
  allowPrivate: false,
  enabled: false,
  allowed: true,
  source: {},
  signIn: '',
  checkedAt: 0,
  tools: [],
  ...o,
})

type Call = { name: string; args: Record<string, unknown> }
let calls: Call[]
const reply = (f: (c: Call) => unknown) =>
  tools.attach(((name: string, args: Record<string, unknown>) => {
    const c = { name, args }
    calls.push(c)
    try {
      return Promise.resolve(f(c))
    } catch (e) {
      return Promise.reject(e)
    }
  }) as never)

beforeEach(() => {
  calls = []
  tools.servers = []
  tools.failures = {}
  tools.checking = {}
})

describe('save', () => {
  it('sends allow only when asked, and never keeps a secret value', async () => {
    reply(() => server())
    const d = emptyDraft()
    const s = await tools.save({ ...d, name: 'Notes', command: 'fake-mcp' }, { env: { API_KEY: 'hunter2-value' }, headers: {} }, true)
    expect(calls[0].args).toMatchObject({ allow: true, secrets: { env: { API_KEY: 'hunter2-value' } } })
    expect(s.env[0].set).toBe(true)
    // What the store holds is what the server answered: names and flags.
    expect(JSON.stringify({ servers: tools.servers, failures: tools.failures })).not.toContain('hunter2')
    expect(tools.servers).toHaveLength(1)
  })

  it('leaves out allow and secrets when there are none', async () => {
    reply(() => server())
    await tools.save(emptyDraft(), undefined, false)
    expect(calls[0].args).not.toHaveProperty('allow')
    expect(calls[0].args).not.toHaveProperty('secrets')
  })

  it('keeps nothing from a save that failed', async () => {
    reply(() => {
      throw new CommandError('SECRETS_UNAVAILABLE', 'no store')
    })
    await expect(tools.save(emptyDraft(), { env: { A: 'topsecret' }, headers: {} }, true)).rejects.toThrow('no store')
    expect(JSON.stringify(tools.servers) + JSON.stringify(tools.failures)).not.toContain('topsecret')
    expect(tools.servers).toHaveLength(0)
  })
})

describe('check and enable', () => {
  it('holds the failure reason until a check passes', async () => {
    tools.servers = [server()]
    reply(() => {
      throw new CommandError('CHECK_FAILED', 'spawn nope: not found')
    })
    await expect(tools.check('s1')).rejects.toThrow()
    expect(tools.failures.s1).toBe('spawn nope: not found')
    expect(tools.checking.s1).toBeUndefined()
    reply(() => server({ checkedAt: 5 }))
    await tools.check('s1')
    expect(tools.failures.s1).toBeUndefined()
    expect(tools.get('s1')?.checkedAt).toBe(5)
  })

  it('asks for a fresh list only for Check again', async () => {
    tools.servers = [server()]
    reply(() => server({ checkedAt: 5 }))
    await tools.check('s1')
    await tools.check('s1', true)
    expect(calls[0].args).toEqual({ id: 's1' })
    expect(calls[1].args).toEqual({ id: 's1', fresh: true })
  })

  it('keeps the reason when enable is refused for a missing check', async () => {
    tools.servers = [server()]
    reply(() => {
      throw new CommandError('CHECK_FAILED', 'Check it first.')
    })
    await expect(tools.enable('s1', true)).rejects.toThrow()
    expect(tools.failures.s1).toBe('Check it first.')
  })

  it('lists only enabled servers for a new chat', () => {
    tools.servers = [server(), server({ id: 's2', enabled: true })]
    expect(tools.enabled.map((s) => s.id)).toEqual(['s2'])
  })
})

describe('events', () => {
  it('adds, replaces and removes from toolServer events', () => {
    tools.apply('toolServer.changed', server())
    tools.apply('toolServer.changed', server({ enabled: true }))
    expect(tools.servers).toHaveLength(1)
    expect(tools.get('s1')?.enabled).toBe(true)
    tools.failures.s1 = 'x'
    tools.apply('toolServer.removed', { id: 's1' })
    expect(tools.servers).toHaveLength(0)
    expect(tools.failures.s1).toBeUndefined()
  })
})

describe('sign-in', () => {
  it('asks for the page and finishes with the pasted address', async () => {
    tools.servers = [server({ kind: 'http', signIn: 'needed' })]
    reply((c) => (c.name === 'toolServer.signIn' ? { url: 'https://auth/x', redirect: 'http://127.0.0.1:7340/api/toolservers/callback' } : server({ kind: 'http', signIn: 'signedIn' })))
    expect((await tools.signIn('s1')).url).toBe('https://auth/x')
    await tools.signInFinish('s1', 'http://127.0.0.1:7340/api/toolservers/callback?code=c&state=s')
    expect(calls[1]).toEqual({ name: 'toolServer.signInFinish', args: { id: 's1', url: 'http://127.0.0.1:7340/api/toolservers/callback?code=c&state=s' } })
    expect(tools.get('s1')?.signIn).toBe('signedIn')
  })
})

describe('catalog drafts', () => {
  it('uses the entry command line as the server gave it', () => {
    const d = draftOfEntry(
      { name: 'io.x/notes', title: 'Notes', description: '', version: '1.2.0', website: '', repository: '', kinds: [] },
      { kind: 'npm', command: 'npx', args: ['-y', 'notes@1.2.0'], env: [{ name: 'TOKEN', description: 'd', required: true, secret: true }], headers: [], available: true },
    )
    expect(d).toMatchObject({ kind: 'npm', command: 'npx', args: ['-y', 'notes@1.2.0'], source: { registry: 'io.x/notes', version: '1.2.0' } })
    expect(d.env[0]).toMatchObject({ name: 'TOKEN', set: false })
  })
})

describe('load', () => {
  it('does not bring back a server removed while the read was out', async () => {
    const f = globalThis.fetch
    let calls = 0
    globalThis.fetch = (async () => {
      calls++
      if (calls === 1) tools.apply('toolServer.removed', { id: 's1' })
      return new Response(JSON.stringify({ servers: calls === 1 ? [server()] : [], canSaveSecrets: true }), { status: 200 })
    }) as typeof fetch
    try {
      await tools.load()
    } finally {
      globalThis.fetch = f
    }
    expect(tools.servers).toHaveLength(0)
    expect(calls).toBe(2)
  })
})
