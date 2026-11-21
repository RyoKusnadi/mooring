// The socket to one language server through the relay. One text
// frame is one JSON-RPC message. The client library has no reconnect, so this owns it: a fresh
// transport for every socket, and the retry rules for the close codes the relay uses.
// Plain TypeScript: the socket and the clock are injected so tests can drive it.

export const CLOSE_RESTARTED = 4001
export const CLOSE_CRASHED = 4002
/** Waits after the 1st, 2nd and later failures. */
export const RETRY_MS = [1000, 5000, 30_000]
/** A third crash inside this window ends the retries. */
export const CRASH_WINDOW_MS = 2 * 60_000
export const MAX_CRASHES = 3

export interface WireSocket {
  send(data: string): void
  close(code?: number): void
  readyState: number
  onopen: (() => void) | null
  onmessage: ((e: { data: unknown }) => void) | null
  onclose: ((e: { code: number }) => void) | null
  onerror: (() => void) | null
}

export interface Clock {
  setTimeout(fn: () => void, ms: number): unknown
  clearTimeout(handle: unknown): void
}

export interface WireTransport {
  send(message: string): void
  subscribe(handler: (value: string) => void): void
  unsubscribe(handler: (value: string) => void): void
}

export type WireState = 'connecting' | 'open' | 'waiting' | 'failed' | 'stopped'

export interface WireOptions {
  open: () => WireSocket
  /** A socket is open: hand this transport to the client. */
  onUp: (transport: WireTransport) => void
  /** The open socket is gone (restart, crash or lost); the client should be disconnected. */
  onDown: (why: 'restarted' | 'crashed' | 'lost') => void
  /** Retries ended after repeated crashes. */
  onGiveUp: () => void
  onState?: (state: WireState) => void
  clock?: Clock
  now?: () => number
}

const realClock: Clock = {
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
}
const OPEN = 1

export class LspWire {
  state: WireState = 'stopped'
  private socket: WireSocket | null = null
  private timer: unknown = null
  private crashes: number[] = []
  private lost = 0
  private clock: Clock
  private now: () => number

  constructor(private o: WireOptions) {
    this.clock = o.clock ?? realClock
    this.now = o.now ?? Date.now
  }

  /** Opens the socket; does nothing while one is open or a retry is already waiting. */
  start() {
    if (this.socket || this.timer !== null) return
    this.setState('connecting')
    this.connect()
  }

  /** Closes for good (the last tab closed, or the server was stopped). `start` can open it again. */
  stop() {
    this.clock.clearTimeout(this.timer)
    this.timer = null
    this.detach()?.close(1000)
    this.setState('stopped')
  }

  /** Forgets the crash count and opens again (the user asked). */
  restart() {
    this.stop()
    this.crashes = []
    this.lost = 0
    this.start()
  }

  private setState(s: WireState) {
    if (this.state === s) return
    this.state = s
    this.o.onState?.(s)
  }

  private detach(): WireSocket | null {
    const s = this.socket
    this.socket = null
    if (s) s.onopen = s.onmessage = s.onclose = s.onerror = null
    return s
  }

  private connect() {
    this.timer = null
    const s = this.o.open()
    this.socket = s
    const handlers = new Set<(v: string) => void>()
    const transport: WireTransport = {
      send: (m) => {
        if (this.socket !== s || s.readyState !== OPEN) throw new Error('The language server connection is not open.')
        s.send(m)
      },
      subscribe: (h) => void handlers.add(h),
      unsubscribe: (h) => void handlers.delete(h),
    }
    s.onopen = () => {
      if (this.socket !== s) return
      this.lost = 0
      this.setState('open')
      this.o.onUp(transport)
    }
    s.onmessage = (e) => {
      if (this.socket !== s || typeof e.data !== 'string') return
      for (const h of [...handlers]) h(e.data)
    }
    s.onclose = (e) => {
      if (this.socket === s) this.closed(e.code, this.state === 'open')
    }
    s.onerror = () => {
      // The close event follows an error, and handles it.
    }
  }

  private closed(code: number, wasOpen: boolean) {
    this.detach()
    if (code === CLOSE_RESTARTED) {
      if (wasOpen) this.o.onDown('restarted')
      return this.later(0)
    }
    if (code === CLOSE_CRASHED) {
      const now = this.now()
      this.crashes = [...this.crashes.filter((t) => now - t < CRASH_WINDOW_MS), now]
      if (wasOpen) this.o.onDown('crashed')
      if (this.crashes.length >= MAX_CRASHES) {
        this.setState('failed')
        return this.o.onGiveUp()
      }
      return this.later(RETRY_MS[this.crashes.length - 1])
    }
    if (wasOpen) this.o.onDown('lost')
    this.later(RETRY_MS[Math.min(this.lost++, RETRY_MS.length - 1)])
  }

  private later(ms: number) {
    this.setState('waiting')
    this.timer = this.clock.setTimeout(() => {
      this.setState('connecting')
      this.connect()
    }, ms)
  }
}

/** Wraps a client's request so that, once the connection is gone, a request still waiting
 *  resolves to null instead of rejecting: some callers (signature help as you type) have no
 *  error handler, and a crashed server is already reported in the editor bar. */
export function quietWhenGone<C extends { request: (method: string, params: never) => Promise<unknown> }>(client: C, live: () => boolean): void {
  const request = client.request.bind(client)
  client.request = ((method: string, params: never) =>
    request(method, params).catch((e: unknown) => {
      if (live()) throw e
      return null
    })) as C['request']
}
