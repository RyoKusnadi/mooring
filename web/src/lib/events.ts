// The events connection. Plain TypeScript: the socket and the
// clock are injected so tests can drive it without a browser.

export type Status = 'connecting' | 'live' | 'reconnecting'

export interface SocketLike {
  send(data: string): void
  close(): void
  onopen: (() => void) | null
  onmessage: ((e: { data: unknown }) => void) | null
  onclose: (() => void) | null
  onerror: (() => void) | null
}

export interface Clock {
  setTimeout(fn: () => void, ms: number): unknown
  clearTimeout(handle: unknown): void
}

export class CommandError extends Error {
  constructor(
    readonly code: string,
    message: string,
  ) {
    super(message)
  }
}

export const PING_MS = 10_000
export const SILENCE_MS = 25_000
export const BACKOFF_START_MS = 500
export const BACKOFF_MAX_MS = 10_000

export function newCommandId(): string {
  return crypto.randomUUID().replaceAll('-', '')
}

export interface Options {
  createSocket: () => SocketLike
  clock?: Clock
  newId?: () => string
  /** A full state arrived: first connect, new epoch, or a position the server no longer holds. */
  onReset: (snapshot: unknown) => void
  onEvent: (kind: string, data: unknown, pos: number) => void
  onStatus: (status: Status) => void
}

export interface SendOptions {
  id?: string
  onPending?: () => void
}

interface Waiting {
  frame: string
  resolve: (value: unknown) => void
  reject: (err: Error) => void
  onPending?: () => void
}

const realClock: Clock = {
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
}

export class EventsClient {
  status: Status = 'connecting'
  private socket: SocketLike | null = null
  private epoch = ''
  private pos = 0
  private live = false
  private hidden = false
  private stopped = true
  private delay = BACKOFF_START_MS
  private retry: unknown = null
  private ping: unknown = null
  private silence: unknown = null
  private waiting = new Map<string, Waiting>()
  private clock: Clock
  private newId: () => string

  constructor(private opts: Options) {
    this.clock = opts.clock ?? realClock
    this.newId = opts.newId ?? newCommandId
  }

  start() {
    if (!this.stopped) return
    this.stopped = false
    this.connect()
  }

  stop() {
    this.stopped = true
    this.teardown()
    this.clock.clearTimeout(this.retry)
    this.retry = null
    for (const w of this.waiting.values()) w.reject(new CommandError('CLOSED', 'The connection was closed.'))
    this.waiting.clear()
  }

  /** Connects now instead of waiting for the next try ("Retry now"). A live connection is left alone. */
  retryNow() {
    if (this.stopped || this.live) return
    this.teardown()
    this.clock.clearTimeout(this.retry)
    this.delay = BACKOFF_START_MS
    this.connect()
  }

  /** Runs a command once. The same id is resent after every reconnect until a result arrives. */
  send(name: string, args: unknown, o: SendOptions = {}): Promise<unknown> {
    const id = o.id ?? this.newId()
    return new Promise((resolve, reject) => {
      const frame = JSON.stringify({ t: 'cmd', id, name, args })
      this.waiting.set(id, { frame, resolve, reject, onPending: o.onPending })
      if (this.live) this.write(frame)
    })
  }

  setHidden(hidden: boolean) {
    this.hidden = hidden
    if (this.live) this.write(JSON.stringify({ t: 'visibility', hidden }))
  }

  private setStatus(s: Status) {
    if (this.status === s) return
    this.status = s
    this.opts.onStatus(s)
  }

  private write(frame: string) {
    try {
      this.socket?.send(frame)
    } catch {
      // The close handler will follow and the command stays queued.
    }
  }

  private connect() {
    this.retry = null
    const s = this.opts.createSocket()
    this.socket = s
    this.armSilence()
    s.onopen = () => {
      if (this.socket !== s) return
      this.write(JSON.stringify({ t: 'hello', epoch: this.epoch, pos: this.pos }))
      this.armSilence()
    }
    s.onmessage = (e) => {
      if (this.socket !== s) return
      this.armSilence()
      this.receive(e.data)
    }
    s.onclose = () => {
      if (this.socket === s) this.lost()
    }
    s.onerror = () => {
      if (this.socket === s) this.lost()
    }
  }

  private armSilence() {
    this.clock.clearTimeout(this.silence)
    this.silence = this.clock.setTimeout(() => this.lost(), SILENCE_MS)
  }

  private teardown() {
    this.live = false
    this.clock.clearTimeout(this.ping)
    this.clock.clearTimeout(this.silence)
    const s = this.socket
    this.socket = null
    if (s) {
      s.onopen = s.onmessage = s.onclose = s.onerror = null
      try {
        s.close()
      } catch {
        // Already closed.
      }
    }
  }

  private lost() {
    if (this.stopped) return
    this.teardown()
    this.setStatus('reconnecting')
    this.retry = this.clock.setTimeout(() => this.connect(), this.delay)
    this.delay = Math.min(this.delay * 2, BACKOFF_MAX_MS)
  }

  private receive(raw: unknown) {
    let f: Record<string, unknown>
    try {
      f = JSON.parse(String(raw))
    } catch {
      return
    }
    switch (f.t) {
      case 'welcome':
        return this.welcome(f)
      case 'event':
        if (!this.live) return
        return this.event(f)
      case 'result':
        return this.result(f)
      case 'pending':
        return this.waiting.get(String(f.id))?.onPending?.()
    }
  }

  private welcome(f: Record<string, unknown>) {
    this.epoch = String(f.epoch ?? '')
    if (f.snapshot !== undefined && f.snapshot !== null) {
      this.pos = Number(f.pos) || 0
      this.opts.onReset(f.snapshot)
    }
    this.live = true
    this.delay = BACKOFF_START_MS
    this.setStatus('live')
    if (this.hidden) this.write(JSON.stringify({ t: 'visibility', hidden: true }))
    for (const w of this.waiting.values()) this.write(w.frame)
    this.schedulePing()
  }

  private event(f: Record<string, unknown>) {
    const pos = Number(f.pos)
    if (!(pos > this.pos)) return
    this.pos = pos
    this.opts.onEvent(String(f.kind), f.data, pos)
  }

  private result(f: Record<string, unknown>) {
    const id = String(f.id)
    const w = this.waiting.get(id)
    if (!w) return
    this.waiting.delete(id)
    if (f.ok) return w.resolve(f.value)
    const e = (f.error ?? {}) as { code?: string; message?: string }
    w.reject(new CommandError(e.code ?? 'INTERNAL', e.message ?? 'The server could not do that.'))
  }

  private schedulePing() {
    this.clock.clearTimeout(this.ping)
    this.ping = this.clock.setTimeout(() => {
      this.write(JSON.stringify({ t: 'ping' }))
      this.schedulePing()
    }, PING_MS)
  }
}
