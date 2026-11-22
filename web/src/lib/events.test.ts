import { beforeEach, describe, expect, it } from 'vitest'
import { CommandError, EventsClient, type Clock, type SocketLike, type Status } from './events'

class FakeSocket implements SocketLike {
  sent: Record<string, unknown>[] = []
  closed = false
  onopen: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  send(data: string) {
    this.sent.push(JSON.parse(data))
  }
  close() {
    this.closed = true
  }
  open() {
    this.onopen?.()
  }
  serve(frame: unknown) {
    this.onmessage?.({ data: JSON.stringify(frame) })
  }
  drop() {
    this.onclose?.()
  }
  frames(t: string) {
    return this.sent.filter((f) => f.t === t)
  }
}

class FakeClock implements Clock {
  now = 0
  private timers: { at: number; fn: () => void; handle: number }[] = []
  private next = 1
  setTimeout(fn: () => void, ms: number) {
    const handle = this.next++
    this.timers.push({ at: this.now + ms, fn, handle })
    return handle
  }
  clearTimeout(handle: unknown) {
    this.timers = this.timers.filter((t) => t.handle !== handle)
  }
  advance(ms: number) {
    const end = this.now + ms
    for (;;) {
      this.timers.sort((a, b) => a.at - b.at)
      const t = this.timers[0]
      if (!t || t.at > end) break
      this.timers.shift()
      this.now = t.at
      t.fn()
    }
    this.now = end
  }
}

const snapshot = { home: '/h', projects: [], layouts: {} }

let sockets: FakeSocket[]
let clock: FakeClock
let statuses: Status[]
let resets: unknown[]
let events: [string, unknown, number][]
let client: EventsClient
let ids: number

beforeEach(() => {
  sockets = []
  clock = new FakeClock()
  statuses = []
  resets = []
  events = []
  ids = 0
  client = new EventsClient({
    createSocket: () => {
      const s = new FakeSocket()
      sockets.push(s)
      return s
    },
    clock,
    newId: () => `c${++ids}`,
    onReset: (s) => resets.push(s),
    onEvent: (k, d, p) => events.push([k, d, p]),
    onStatus: (s) => statuses.push(s),
  })
})

function goLive(withSnapshot = true, epoch = 'e1', pos = 5) {
  const s = sockets[sockets.length - 1]
  s.open()
  s.serve({ t: 'welcome', epoch, pos, ...(withSnapshot ? { snapshot } : {}) })
  return s
}

describe('connect and resume', () => {
  it('sends hello first with no epoch, then resets from the snapshot', () => {
    client.start()
    const s = goLive()
    expect(s.sent[0]).toEqual({ t: 'hello', epoch: '', pos: 0 })
    expect(resets).toEqual([snapshot])
    expect(client.status).toBe('live')
  })

  it('applies events in order and skips repeats', () => {
    client.start()
    const s = goLive()
    s.serve({ t: 'event', pos: 6, kind: 'a', data: 1 })
    s.serve({ t: 'event', pos: 7, kind: 'b', data: 2 })
    s.serve({ t: 'event', pos: 7, kind: 'b', data: 2 })
    s.serve({ t: 'event', pos: 6, kind: 'a', data: 1 })
    expect(events.map((e) => e[0])).toEqual(['a', 'b'])
  })

  it('resumes without a snapshot using the last epoch and pos', () => {
    client.start()
    const first = goLive()
    first.serve({ t: 'event', pos: 6, kind: 'a', data: 1 })
    first.drop()
    expect(client.status).toBe('reconnecting')
    clock.advance(500)
    const second = sockets[1]
    second.open()
    expect(second.sent[0]).toEqual({ t: 'hello', epoch: 'e1', pos: 6 })
    second.serve({ t: 'welcome', epoch: 'e1', pos: 8 })
    second.serve({ t: 'event', pos: 7, kind: 'b', data: 2 })
    second.serve({ t: 'event', pos: 8, kind: 'c', data: 3 })
    expect(resets).toHaveLength(1)
    expect(events.map((e) => e[0])).toEqual(['a', 'b', 'c'])
    expect(client.status).toBe('live')
  })

  it('resets from a snapshot when the server asks, and takes its pos', () => {
    client.start()
    const first = goLive()
    first.serve({ t: 'event', pos: 6, kind: 'a', data: 1 })
    first.drop()
    clock.advance(500)
    const second = sockets[1]
    second.open()
    second.serve({ t: 'welcome', epoch: 'e2', pos: 2, snapshot: { ...snapshot, home: '/new' } })
    expect(resets).toHaveLength(2)
    second.serve({ t: 'event', pos: 3, kind: 'z', data: 0 })
    expect(events.map((e) => e[0])).toEqual(['a', 'z'])
    second.drop()
    clock.advance(500)
    sockets[2].open()
    expect(sockets[2].sent[0]).toEqual({ t: 'hello', epoch: 'e2', pos: 3 })
  })
})

describe('commands', () => {
  it('resolves with the value and rejects with the error code', async () => {
    client.start()
    const s = goLive()
    const ok = client.send('fs.list', { path: '/h' })
    const bad = client.send('project.add', { path: '/x' })
    expect(s.frames('cmd').map((f) => f.id)).toEqual(['c1', 'c2'])
    s.serve({ t: 'result', id: 'c1', ok: true, value: { path: '/h' } })
    s.serve({ t: 'result', id: 'c2', ok: false, error: { code: 'NOT_FOUND', message: 'No such folder.' } })
    await expect(ok).resolves.toEqual({ path: '/h' })
    const err = await bad.catch((e) => e)
    expect(err).toBeInstanceOf(CommandError)
    expect(err).toMatchObject({ code: 'NOT_FOUND', message: 'No such folder.' })
  })

  it('queues a command sent before the connection is live', () => {
    client.start()
    void client.send('project.remove', { id: 'p' })
    const s = sockets[0]
    s.open()
    expect(s.frames('cmd')).toHaveLength(0)
    s.serve({ t: 'welcome', epoch: 'e1', pos: 0, snapshot })
    expect(s.frames('cmd')).toEqual([{ t: 'cmd', id: 'c1', name: 'project.remove', args: { id: 'p' } }])
  })

  it('resends the same id after a drop mid-command, and runs it once on the client', async () => {
    client.start()
    const first = goLive()
    const p = client.send('project.add', { path: '/x' })
    expect(first.frames('cmd')).toHaveLength(1)
    first.drop()
    clock.advance(500)
    const second = goLive(false)
    expect(second.frames('cmd')).toEqual(first.frames('cmd'))
    second.serve({ t: 'result', id: 'c1', ok: true, value: { id: 'p1' } })
    await expect(p).resolves.toEqual({ id: 'p1' })
    second.drop()
    clock.advance(1000)
    const third = goLive(false)
    expect(third.frames('cmd')).toHaveLength(0)
  })

  it('keeps resending while no result has arrived, and reports pending', () => {
    client.start()
    const s = goLive()
    let pending = 0
    void client.send('project.clone', { url: 'u', parent: '/h' }, { id: 'clone1', onPending: () => pending++ })
    s.serve({ t: 'pending', id: 'clone1' })
    expect(pending).toBe(1)
    s.drop()
    clock.advance(500)
    const next = goLive(false)
    expect(next.frames('cmd').map((f) => f.id)).toEqual(['clone1'])
  })

  it('rejects open commands when stopped', async () => {
    client.start()
    goLive()
    const p = client.send('layout.save', {})
    client.stop()
    await expect(p).rejects.toMatchObject({ code: 'CLOSED' })
  })
})

describe('keepalive, visibility, backoff', () => {
  it('pings every 10 seconds once live', () => {
    client.start()
    const s = goLive()
    clock.advance(10_000)
    expect(s.frames('ping')).toHaveLength(1)
    s.serve({ t: 'pong' })
    clock.advance(10_000)
    expect(s.frames('ping')).toHaveLength(2)
  })

  it('gives up on a silent server after 25 seconds and reconnects', () => {
    client.start()
    const s = goLive()
    clock.advance(24_999)
    expect(sockets).toHaveLength(1)
    clock.advance(1)
    expect(s.closed).toBe(true)
    expect(client.status).toBe('reconnecting')
    clock.advance(500)
    expect(sockets).toHaveLength(2)
  })

  it('a pong keeps the connection alive', () => {
    client.start()
    const s = goLive()
    for (let i = 0; i < 5; i++) {
      clock.advance(10_000)
      s.serve({ t: 'pong' })
    }
    expect(sockets).toHaveLength(1)
    expect(client.status).toBe('live')
  })

  it('sends visibility changes, and again after a reconnect while hidden', () => {
    client.start()
    const s = goLive()
    client.setHidden(true)
    expect(s.frames('visibility')).toEqual([{ t: 'visibility', hidden: true }])
    s.drop()
    clock.advance(500)
    const next = goLive(false)
    expect(next.frames('visibility')).toEqual([{ t: 'visibility', hidden: true }])
  })

  it('backs off 0.5 s doubling to 10 s, and resets after a welcome', () => {
    client.start()
    const waits: number[] = []
    for (let i = 0; i < 7; i++) {
      sockets[sockets.length - 1].drop()
      let waited = 0
      const before = sockets.length
      while (sockets.length === before) {
        clock.advance(100)
        waited += 100
      }
      waits.push(waited)
    }
    expect(waits).toEqual([500, 1000, 2000, 4000, 8000, 10_000, 10_000])
    goLive()
    sockets[sockets.length - 1].drop()
    clock.advance(500)
    expect(sockets.length).toBe(9)
  })

  it('reports connecting, then live, then reconnecting', () => {
    client.start()
    expect(client.status).toBe('connecting')
    const s = goLive()
    s.drop()
    expect(statuses).toEqual(['live', 'reconnecting'])
  })

  it('ignores frames from a socket it already dropped', () => {
    client.start()
    const first = goLive()
    first.drop()
    clock.advance(500)
    first.onmessage?.({ data: JSON.stringify({ t: 'event', pos: 99, kind: 'x', data: 0 }) })
    expect(events).toHaveLength(0)
  })
})
