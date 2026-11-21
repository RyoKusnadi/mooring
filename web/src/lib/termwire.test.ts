import { beforeEach, describe, expect, it } from 'vitest'
import type { Clock } from './events'
import { ACK_BYTES, TerminalConnection, type StartInfo, type TermSocket, type WireState } from './termwire'

class FakeSocket implements TermSocket {
  binaryType = ''
  sent: (string | Uint8Array)[] = []
  closed = false
  readyState = 1
  onopen: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  send(d: string | Uint8Array) {
    this.sent.push(d)
  }
  close() {
    this.closed = true
  }
  json(t: string) {
    return this.sent.filter((s): s is string => typeof s === 'string').map((s) => JSON.parse(s)).filter((f) => f.t === t)
  }
  serve(f: unknown) {
    this.onmessage?.({ data: JSON.stringify(f) })
  }
  out(pos: number, text: string) {
    const body = new TextEncoder().encode(text)
    const buf = new Uint8Array(8 + body.length)
    new DataView(buf.buffer).setBigUint64(0, BigInt(pos))
    buf.set(body, 8)
    this.onmessage?.({ data: buf.buffer })
  }
}

class FakeClock implements Clock {
  now = 0
  private timers: { at: number; fn: () => void; h: number }[] = []
  private n = 1
  setTimeout(fn: () => void, ms: number) {
    this.timers.push({ at: this.now + ms, fn, h: this.n })
    return this.n++
  }
  clearTimeout(h: unknown) {
    this.timers = this.timers.filter((t) => t.h !== h)
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

let sockets: FakeSocket[]
let clock: FakeClock
let starts: StartInfo[]
let data: [string, number][]
let states: WireState[]
let conn: TerminalConnection

beforeEach(() => {
  sockets = []
  clock = new FakeClock()
  starts = []
  data = []
  states = []
  conn = new TerminalConnection({
    clock,
    createSocket: () => {
      const s = new FakeSocket()
      sockets.push(s)
      return s
    },
    onStart: (s) => starts.push(s),
    onData: (b, p) => data.push([new TextDecoder().decode(b), p]),
    onSize: () => {},
    onExit: () => {},
    onState: (s) => states.push(s),
  })
})

function join(s: FakeSocket, extra: object = {}) {
  s.onopen?.()
  s.serve({ t: 'hello', epoch: 'e1', pos: 0, reset: false, cols: 80, rows: 24, others: 0, ...extra })
}

describe('TerminalConnection', () => {
  it('says hello with the epoch and position, then goes live', () => {
    conn.start()
    join(sockets[0])
    expect(sockets[0].json('hello')[0]).toMatchObject({ epoch: '', pos: 0 })
    expect(states).toEqual(['live'])
    expect(starts[0]).toMatchObject({ hello: true, others: 0, cols: 80 })
  })

  it('waits for a connecting socket to open before closing it', () => {
    conn.start()
    sockets[0].readyState = 0
    conn.stop()
    expect(sockets[0].closed).toBe(false)
    sockets[0].onopen?.()
    expect(sockets[0].closed).toBe(true)
  })

  it('counts output and resumes from the last position after stop and start', () => {
    conn.start()
    join(sockets[0])
    sockets[0].out(0, 'abc')
    sockets[0].out(3, 'de')
    expect(conn.pos).toBe(5)
    conn.stop()
    conn.start()
    sockets[1].onopen?.()
    expect(sockets[1].json('hello')[0]).toMatchObject({ epoch: 'e1', pos: 5 })
  })

  it('reconnects with backoff when the socket drops', () => {
    conn.start()
    join(sockets[0])
    sockets[0].onclose?.()
    expect(states).toEqual(['live', 'reconnecting'])
    clock.advance(10_000)
    expect(sockets).toHaveLength(2)
  })

  it('treats a gap in positions as a lost connection', () => {
    conn.start()
    join(sockets[0])
    sockets[0].out(0, 'abc')
    sockets[0].out(10, 'zz')
    expect(data).toHaveLength(1)
    expect(states.at(-1)).toBe('reconnecting')
  })

  it('acks when idle, or after a window of bytes, and never backwards', () => {
    conn.start()
    join(sockets[0])
    conn.drawn(10, false)
    expect(sockets[0].json('ack')).toHaveLength(0)
    conn.drawn(10, true)
    conn.drawn(10, true)
    conn.drawn(ACK_BYTES + 10, false)
    expect(sockets[0].json('ack').map((a) => a.pos)).toEqual([10, ACK_BYTES + 10])
  })

  it('starts over on a reset frame', () => {
    conn.start()
    join(sockets[0])
    sockets[0].out(0, 'abc')
    sockets[0].serve({ t: 'reset', pos: 100, reset: true, modes: '\x1b[?1049h', cols: 90, rows: 30 })
    expect(conn.pos).toBe(100)
    expect(starts[1]).toMatchObject({ reset: true, hello: false, modes: '\x1b[?1049h', cols: 90 })
    sockets[0].out(100, 'x')
    expect(data.at(-1)).toEqual(['x', 100])
  })

  it('does not reconnect after the shell exited', () => {
    conn.start()
    join(sockets[0])
    sockets[0].serve({ t: 'exit', code: 3 })
    sockets[0].onclose?.()
    clock.advance(60_000)
    expect(sockets).toHaveLength(1)
    expect(states.at(-1)).toBe('ended')
  })

  it('sends input and resize only while live, and splits long input', () => {
    conn.start()
    conn.sendInput('lost')
    join(sockets[0])
    conn.sendInput('ls\n')
    conn.sendResize(100, 40)
    conn.sendInput('a'.repeat(70 * 1024))
    const bins = sockets[0].sent.filter((s) => typeof s !== 'string') as Uint8Array[]
    expect(bins.map((b) => b.length)).toEqual([3, 64 * 1024, 6 * 1024])
    expect(sockets[0].json('resize')[0]).toMatchObject({ cols: 100, rows: 40 })
  })

  it('pings while live and drops a silent socket', () => {
    conn.start()
    join(sockets[0])
    clock.advance(25_000)
    expect(sockets[0].json('ping').length).toBeGreaterThan(0)
    clock.advance(60_000)
    expect(states.at(-1)).toBe('reconnecting')
  })
})
