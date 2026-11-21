import { beforeEach, describe, expect, it } from 'vitest'
import type { Account } from './accountapi'
import { accounts } from './accountstore.svelte'
import { limited, percentText, resetText } from './accountfmt'

const usage = { source: 'unknown' as const, windows: [], limitHit: 0, limitResetsAt: 0, updatedAt: 0 }
const acc = (o: Partial<Account> = {}): Account => ({ id: 'a1', agent: 'claude', name: 'Work', dir: '/x', managed: true, isDefault: false, login: 'unknown', usage, ...o })
const builtin = acc({ id: 'default:claude', name: 'Default', managed: false, isDefault: true })

type Call = { name: string; args: Record<string, unknown> }
let calls: Call[]
const reply = (f: (c: Call) => unknown) =>
  accounts.attach(((name: string, args: Record<string, unknown>) => {
    const c = { name, args }
    calls.push(c)
    return Promise.resolve(f(c))
  }) as never)

beforeEach(() => {
  calls = []
  accounts.accounts = [builtin]
  accounts.chosen = {}
})

describe('events', () => {
  it('adds and replaces an account, and keeps one default per agent', () => {
    accounts.apply('account.changed', acc())
    accounts.apply('account.changed', acc({ name: 'Work 2' }))
    expect(accounts.of('claude').map((a) => a.name)).toEqual(['Default', 'Work 2'])
    accounts.apply('account.changed', acc({ isDefault: true }))
    expect(accounts.of('claude').filter((a) => a.isDefault).map((a) => a.id)).toEqual(['a1'])
  })

  it('removes an account and the project choices that named it', () => {
    accounts.accounts = [builtin, acc()]
    accounts.chosen = { p1: { claude: 'a1' } }
    accounts.apply('account.removed', { id: 'a1' })
    expect(accounts.get('a1')).toBeUndefined()
    expect(accounts.forProject('p1', 'claude')?.id).toBe('default:claude')
  })

  it('follows a project account change', () => {
    accounts.accounts = [builtin, acc()]
    accounts.apply('project.accountChanged', { projectId: 'p1', agent: 'claude', accountId: 'a1' })
    expect(accounts.forProject('p1', 'claude')?.id).toBe('a1')
    expect(accounts.forProject('p2', 'claude')?.id).toBe('default:claude')
  })
})

describe('commands', () => {
  it('saves with the account object and keeps the answer', async () => {
    reply(() => acc())
    await accounts.save({ agent: 'claude', name: 'Work' })
    expect(calls[0]).toEqual({ name: 'account.save', args: { account: { agent: 'claude', name: 'Work' } } })
    expect(accounts.get('a1')?.name).toBe('Work')
  })

  it('moves a project and reports the chats that restarted', async () => {
    reply(() => ({ restarted: ['c1'] }))
    const r = await accounts.setProjectAccount('p1', 'claude', 'a1')
    expect(r).toEqual(['c1'])
    expect(calls[0].args).toEqual({ projectId: 'p1', agent: 'claude', accountId: 'a1' })
    expect(accounts.chosen.p1.claude).toBe('a1')
  })

  it('does not change anything when the move fails', async () => {
    accounts.attach((() => Promise.reject(new Error('BUSY'))) as never)
    await expect(accounts.setProjectAccount('p1', 'claude', 'a1')).rejects.toThrow()
    expect(accounts.chosen.p1).toBeUndefined()
  })

  it('returns the terminal that sign in opened', async () => {
    reply(() => ({ terminalId: 't9' }))
    expect(await accounts.login('a1', 'p1')).toBe('t9')
    expect(calls[0]).toEqual({ name: 'account.login', args: { id: 'a1', projectId: 'p1' } })
  })

  it('moves the default mark on set default', async () => {
    accounts.accounts = [builtin, acc()]
    reply(() => acc({ isDefault: true }))
    await accounts.setDefault('a1')
    expect(accounts.of('claude').map((a) => a.isDefault)).toEqual([false, true])
  })
})

describe('agents', () => {
  it('lists only agents that have an account', () => {
    expect(accounts.agents).toEqual(['claude'])
    expect(accounts.has('gemini')).toBe(false)
  })
})

describe('words', () => {
  it('shows unknown for a missing percent, never a number', () => {
    expect(percentText({ name: 'five_hour', usedPercent: null, resetsAt: 0, status: '' })).toBe('unknown')
    expect(percentText({ name: 'five_hour', usedPercent: 41.6, resetsAt: 0, status: '' })).toBe('42%')
  })
  it('says when the reset time is not known', () => {
    expect(resetText(0)).toBe('reset time unknown')
  })
  it('keeps a limit until its reset passes, or until a turn runs when the reset is unknown', () => {
    const now = 1000
    expect(limited({ ...usage, limitHit: 5, limitResetsAt: 2000 }, now)).toBe(true)
    expect(limited({ ...usage, limitHit: 5, limitResetsAt: 500 }, now)).toBe(false)
    expect(limited({ ...usage, limitHit: 5, limitResetsAt: 0 }, now)).toBe(true)
    expect(limited(usage, now)).toBe(false)
  })
})
