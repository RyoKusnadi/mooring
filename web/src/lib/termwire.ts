// The terminal socket. Plain TypeScript: the
// socket and the clock are injected so tests can drive it without a browser.
import { BACKOFF_MAX_MS, BACKOFF_START_MS, PING_MS, SILENCE_MS, type Clock } from './events'

export interface TermSocket {
  binaryType: string
  /** 0 while connecting (a real WebSocket); fakes may leave it out. */
  readyState?: number
  send(data: string | Uint8Array): void
  close(): void
  onopen: (() => void) | null
  onmessage: ((e: { data: unknown }) => void) | null
  onclose: (() => void) | null
  onerror: (() => void) | null
}

export type WireState = 'connecting' | 'live' | 'reconnecting' | 'ended'

/** The start of a stream: first connect (hello) or a reset. The view clears the screen and starts over. */
export interface StartInfo {
  pos: number
  reset: boolean
  modes: string
  cols: number
  rows: number
  /** First start of this connection (as opposed to a reset in the middle of it). */
  hello: boolean
  /** Other clients attached when this one joined (hello only). */
  others: number
}

export interface WireOptions {
  createSocket: () => TermSocket
  clock?: Clock
  onStart: (s: StartInfo) => void
  /** Output starting at byte position `pos`. The view calls `drawn` once it has parsed it. */
  onData: (bytes: Uint8Array, pos: number) => void
  onSize: (cols: number, rows: number) => void
  onExit: (code: number) => void
  onState: (s: WireState) => void
}

/** Ack after this many drawn bytes, or when the view has nothing left to draw. */
export const ACK_BYTES = 64 * 1024
const MAX_INPUT = 64 * 1024

const realClock: Clock = {
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
}

export class TerminalConnection {
  state: WireState = 'connecting'
  private socket: TermSocket | null = null
  private epoch = ''
  private _pos = 0
  private acked = 0
  private live = false
  private stopped = true
  private delay = BACKOFF_START_MS
  private retry: unknown = null
  private ping: unknown = null
  private silence: unknown = null
  private clock: Clock
  private encoder = new TextEncoder()

  constructor(private opts: WireOptions) {
    this.clock = opts.clock ?? realClock
  }

  /** Bytes of output received so far (the position a reconnect resumes from). */
  get pos() {
    return this._pos
  }

  start() {
    if (!this.stopped) return
    this.stopped = false
    this.connect()
  }

  /** Closes the socket but keeps the position, so `start` resumes where this left off. */
  stop() {
    this.stopped = true
    this.teardown()
    this.clock.clearTimeout(this.retry)
    this.retry = null
  }

  /** The network or the tab came back: a connection waiting out its backoff tries now. */
  wake() {
    if (this.stopped || this.retry === null) return
    this.clock.clearTimeout(this.retry)
    this.delay = BACKOFF_START_MS
    this.connect()
  }

  sendInput(data: string | Uint8Array) {
    const bytes = typeof data === 'string' ? this.encoder.encode(data) : data
    if (!this.live) return
    for (let i = 0; i < bytes.length; i += MAX_INPUT) this.write(bytes.subarray(i, i + MAX_INPUT))
  }

  sendResize(cols: number, rows: number) {
    if (this.live) this.write(JSON.stringify({ t: 'resize', cols, rows }))
  }

  /** The view has parsed output up to `end`. `idle`: nothing else is waiting to be parsed. */
  drawn(end: number, idle: boolean) {
    if (end > this.acked && (idle || end - this.acked >= ACK_BYTES)) {
      this.acked = end
      if (this.live) this.write(JSON.stringify({ t: 'ack', pos: end }))
    }
  }

  private setState(s: WireState) {
    if (this.state === s) return
    this.state = s
    this.opts.onState(s)
  }

  private write(frame: string | Uint8Array) {
    try {
      this.socket?.send(frame)
    } catch {
      // The close handler follows.
    }
  }

  private connect() {
    this.retry = null
    const s = this.opts.createSocket()
    s.binaryType = 'arraybuffer'
    this.socket = s
    this.armSilence()
    s.onopen = () => {
      if (this.socket !== s) return
      this.write(JSON.stringify({ t: 'hello', epoch: this.epoch, pos: this._pos }))
      this.armSilence()
    }
    s.onmessage = (e) => {
      if (this.socket !== s) return
      this.armSilence()
      this.receive(e.data)
    }
    s.onclose = s.onerror = () => {
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
      s.onmessage = s.onclose = s.onerror = null
      s.onopen = null
      try {
        // Closing while CONNECTING makes Chrome log a warning; close once it opens instead.
        if (s.readyState === 0) s.onopen = () => s.close()
        else s.close()
      } catch {
        // Already closed.
      }
    }
  }

  private lost() {
    if (this.stopped) return
    this.teardown()
    if (this.state === 'ended') return
    this.setState('reconnecting')
    this.retry = this.clock.setTimeout(() => this.connect(), this.delay)
    this.delay = Math.min(this.delay * 2, BACKOFF_MAX_MS)
  }

  private receive(raw: unknown) {
    if (raw instanceof ArrayBuffer) return this.output(new Uint8Array(raw))
    let f: Record<string, unknown>
    try {
      f = JSON.parse(String(raw))
    } catch {
      return
    }
    switch (f.t) {
      case 'hello':
      case 'reset':
        return this.begin(f)
      case 'size':
        return this.opts.onSize(Number(f.cols), Number(f.rows))
      case 'exit':
        this.setState('ended')
        return this.opts.onExit(Number(f.code) || 0)
    }
  }

  private begin(f: Record<string, unknown>) {
    const hello = f.t === 'hello'
    if (hello) this.epoch = String(f.epoch ?? '')
    this._pos = Number(f.pos) || 0
    this.acked = Math.max(this.acked, this._pos)
    if (hello) {
      this.live = true
      this.delay = BACKOFF_START_MS
      if (this.state !== 'ended') this.setState('live')
      this.schedulePing()
    }
    this.opts.onStart({
      pos: this._pos,
      reset: f.reset === true,
      modes: String(f.modes ?? ''),
      cols: Number(f.cols) || 80,
      rows: Number(f.rows) || 24,
      hello,
      others: Number(f.others) || 0,
    })
    if (hello && f.exited === true) {
      this.setState('ended')
      this.opts.onExit(Number(f.code) || 0)
    }
  }

  private output(buf: Uint8Array) {
    if (buf.length <= 8) return
    const pos = Number(new DataView(buf.buffer, buf.byteOffset).getBigUint64(0))
    if (pos !== this._pos) return this.lost() // a gap: ask again from what was drawn
    const bytes = buf.subarray(8)
    this._pos += bytes.length
    this.opts.onData(bytes, pos)
  }

  private schedulePing() {
    this.clock.clearTimeout(this.ping)
    this.ping = this.clock.setTimeout(() => {
      this.write(JSON.stringify({ t: 'ping' }))
      this.schedulePing()
    }, PING_MS)
  }
}
