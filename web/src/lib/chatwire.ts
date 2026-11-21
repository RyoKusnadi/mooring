// The chat socket. Plain TypeScript: the socket and the
// clock are injected so tests can drive it without a browser.
import type { Chip, OutputRef } from './tokenfmt'
import { BACKOFF_MAX_MS, BACKOFF_START_MS, PING_MS, SILENCE_MS, closeSocket, type Clock, type SocketLike } from './events'

export type WireState = 'connecting' | 'live' | 'reconnecting'

export interface ChatMessage {
  id: number
  kind: string
  at: number
  text?: string
  code?: string
  toolId?: string
  title?: string
  toolKind?: string
  status?: string
  rawInput?: unknown
  locations?: { path?: string; line?: number }[]
  content?: unknown[]
  truncated?: boolean
  requestId?: string
  options?: { id: string; name: string; kind: string }[]
  state?: string
  chosen?: string
  entries?: { content: string; status: string; priority?: string }[]
  /** tool: raw outputs of commands the agent ran through Mooring. */
  outputs?: OutputRef[]
  /** user: set to "summary" for the prompt that asked for a summary. */
  job?: string
  /** user: what was attached, without the text. */
  attachments?: Chip[]
  /** context: the chat whose summary this chat started from. */
  fromChat?: string
}

export interface Welcome {
  /** The client must replace what it has with `messages`. */
  reset: boolean
  pos: number
  messages: ChatMessage[]
  info: unknown
}

export interface ChatWireOptions {
  createSocket: () => SocketLike
  clock?: Clock
  onWelcome: (w: Welcome) => void
  onSet: (id: number, msg: ChatMessage) => void
  onAppend: (id: number, text: string) => void
  onState: (s: WireState) => void
}

const realClock: Clock = {
  setTimeout: (fn, ms) => setTimeout(fn, ms),
  clearTimeout: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
}

export class ChatConnection {
  state: WireState = 'connecting'
  private socket: SocketLike | null = null
  private epoch = ''
  private _pos = 0
  private live = false
  private stopped = true
  private delay = BACKOFF_START_MS
  private retry: unknown = null
  private ping: unknown = null
  private silence: unknown = null
  private clock: Clock

  constructor(private opts: ChatWireOptions) {
    this.clock = opts.clock ?? realClock
  }

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

  private setState(s: WireState) {
    if (this.state === s) return
    this.state = s
    this.opts.onState(s)
  }

  private write(frame: string) {
    try {
      this.socket?.send(frame)
    } catch {
      // The close handler follows.
    }
  }

  private connect() {
    this.retry = null
    const s = this.opts.createSocket()
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
    if (s) closeSocket(s)
  }

  private lost() {
    if (this.stopped) return
    this.teardown()
    this.setState('reconnecting')
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
    if (f.t === 'welcome') return this.welcome(f)
    if (f.t === 'event' && this.live) return this.event(f)
  }

  private welcome(f: Record<string, unknown>) {
    this.epoch = String(f.epoch ?? '')
    this._pos = Number(f.pos) || 0
    const snap = f.snapshot as { messages?: ChatMessage[]; info?: unknown } | null | undefined
    this.live = true
    this.delay = BACKOFF_START_MS
    this.opts.onWelcome({ reset: !!snap, pos: this._pos, messages: snap?.messages ?? [], info: snap?.info ?? null })
    this.setState('live')
    this.schedulePing()
  }

  private event(f: Record<string, unknown>) {
    const pos = Number(f.pos)
    if (pos <= this._pos) return
    // A missing number means a lost event: ask again from the last one applied.
    if (pos !== this._pos + 1) return this.lost()
    this._pos = pos
    const d = (f.data ?? {}) as { id?: number; msg?: ChatMessage; text?: string }
    if (typeof d.id !== 'number') return
    if (f.kind === 'set' && d.msg) this.opts.onSet(d.id, d.msg)
    else if (f.kind === 'append') this.opts.onAppend(d.id, String(d.text ?? ''))
  }

  private schedulePing() {
    this.clock.clearTimeout(this.ping)
    this.ping = this.clock.setTimeout(() => {
      this.write(JSON.stringify({ t: 'ping' }))
      this.schedulePing()
    }, PING_MS)
  }
}
