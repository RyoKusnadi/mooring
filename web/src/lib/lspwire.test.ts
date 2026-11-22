import { describe, expect, it } from 'vitest'
import { CLOSE_CRASHED, CLOSE_RESTARTED, LspWire, quietWhenGone, RETRY_MS, type Clock, type WireSocket, type WireTransport } from './lspwire'

class FakeSocket implements WireSocket {
  readyState = 0
  sent: string[] = []
  closed: number | undefined
  onopen: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  onclose: ((e: { code: number }) => void) | null = null
  onerror: (() => void) | null = null
  send(d: string) {
    this.sent.push(d)
  }
  close(code?: number) {
    this.closed = code
  }
  open() {
    this.readyState = 1
    this.onopen?.()
  }
  drop(code: number) {
    this.readyState = 3
    this.onclose?.({ code })
  }
}

function setup() {
  const sockets: FakeSocket[] = []
  const timers: { fn: () => void; ms: number; live: boolean }[] = []
  const clock: Clock = {
    setTimeout: (fn, ms) => timers.push({ fn, ms, live: true }) - 1,
    clearTimeout: (h) => {
      if (typeof h === 'number' && timers[h]) timers[h].live = false
    },
  }
  const events: string[] = []
  const ups: WireTransport[] = []
  let now = 0
  const wire = new LspWire({
    open: () => sockets[sockets.push(new FakeSocket()) - 1],
    onUp: (t) => (ups.push(t), events.push('up')),
    onDown: (why) => events.push('down:' + why),
    onGiveUp: () => events.push('giveup'),
    clock,
    now: () => now,
  })
  const fire = () => {
    const t = timers.filter((x) => x.live).pop()
    if (!t) throw new Error('no timer waiting')
    t.live = false
    t.fn()
    return t.ms
  }
  return { wire, sockets, events, ups, fire, timers, tick: (ms: number) => (now += ms) }
}

describe('LspWire', () => {
  it('hands out a transport that sends text and passes messages to subscribers', () => {
    const { wire, sockets, ups } = setup()
    wire.start()
    sockets[0].open()
    const got: string[] = []
    ups[0].subscribe((m) => got.push(m))
    sockets[0].onmessage?.({ data: '{"id":1}' })
    sockets[0].onmessage?.({ data: new ArrayBuffer(1) })
    ups[0].send('{"x":1}')
    expect(got).toEqual(['{"id":1}'])
    expect(sockets[0].sent).toEqual(['{"x":1}'])
  })

  it('throws from send when the socket is not open', () => {
    const { wire, sockets, ups } = setup()
    wire.start()
    sockets[0].open()
    sockets[0].drop(1006)
    expect(() => ups[0].send('{}')).toThrow()
  })

  it('reconnects at once after a restart (4001) with a new transport', () => {
    const { wire, sockets, events, fire, ups } = setup()
    wire.start()
    sockets[0].open()
    sockets[0].drop(CLOSE_RESTARTED)
    expect(events).toEqual(['up', 'down:restarted'])
    expect(fire()).toBe(0)
    sockets[1].open()
    expect(ups).toHaveLength(2)
    expect(ups[1]).not.toBe(ups[0])
  })

  it('backs off 1 s, 5 s, 30 s after lost connections and resets once open', () => {
    const { wire, sockets, fire } = setup()
    wire.start()
    sockets[0].drop(1006)
    expect(fire()).toBe(RETRY_MS[0])
    sockets[1].drop(1006)
    expect(fire()).toBe(RETRY_MS[1])
    sockets[2].drop(1006)
    expect(fire()).toBe(RETRY_MS[2])
    sockets[3].drop(1006)
    expect(fire()).toBe(RETRY_MS[2])
    sockets[4].open()
    sockets[4].drop(1006)
    expect(fire()).toBe(RETRY_MS[0])
  })

  it('retries a crash (4002) after 1 s, then 5 s, and gives up on the third inside 2 minutes', () => {
    const { wire, sockets, events, fire, tick } = setup()
    wire.start()
    sockets[0].open()
    sockets[0].drop(CLOSE_CRASHED)
    expect(fire()).toBe(1000)
    sockets[1].open()
    tick(10_000)
    sockets[1].drop(CLOSE_CRASHED)
    expect(fire()).toBe(5000)
    sockets[2].open()
    tick(10_000)
    sockets[2].drop(CLOSE_CRASHED)
    expect(events.at(-1)).toBe('giveup')
    expect(wire.state).toBe('failed')
    expect(sockets).toHaveLength(3)
  })

  it('forgets old crashes after 2 minutes', () => {
    const { wire, sockets, events, fire, tick } = setup()
    wire.start()
    sockets[0].drop(CLOSE_CRASHED)
    fire()
    tick(130_000)
    sockets[1].drop(CLOSE_CRASHED)
    fire()
    tick(130_000)
    sockets[2].drop(CLOSE_CRASHED)
    expect(events).not.toContain('giveup')
  })

  it('stop closes the socket and cancels the retry; start opens again', () => {
    const { wire, sockets, fire } = setup()
    wire.start()
    sockets[0].drop(1006)
    wire.stop()
    expect(() => fire()).toThrow()
    wire.start()
    expect(sockets).toHaveLength(2)
    sockets[1].open()
    wire.stop()
    expect(sockets[1].closed).toBe(1000)
    sockets[1].drop(1005)
    expect(wire.state).toBe('stopped')
  })

  it('ignores a second start while a socket is open or a retry is waiting', () => {
    const { wire, sockets } = setup()
    wire.start()
    wire.start()
    expect(sockets).toHaveLength(1)
    sockets[0].drop(1006)
    wire.start()
    expect(sockets).toHaveLength(1)
  })
})

describe('quietWhenGone', () => {
  it('turns a late failure into null once the connection is gone, and keeps it while live', async () => {
    let live = true
    const client = { request: (_m: string, _p: never): Promise<unknown> => Promise.reject(new Error('Request timed out')) }
    quietWhenGone(client, () => live)
    await expect(client.request('textDocument/hover', undefined as never)).rejects.toThrow('Request timed out')
    live = false
    await expect(client.request('textDocument/hover', undefined as never)).resolves.toBeNull()
  })
})
