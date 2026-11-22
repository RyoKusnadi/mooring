// One chat's messages as the server streams them. Messages sit in a SvelteMap so a
// streamed chunk wakes only the row it belongs to, not the list.
import { SvelteMap } from 'svelte/reactivity'
import type { SocketLike } from './events'
import { ChatConnection, type ChatMessage, type WireState, type Welcome } from './chatwire'
import { onWake } from './wake'

export class ChatFeed {
  readonly msgs = new SvelteMap<number, ChatMessage>()
  /** Messages are numbered 0..count-1. */
  count = $state(0)
  wire = $state<WireState>('connecting')
  /** The server said the agent could not be switched to a mode that asks first. */
  noAsk = $state(false)
  /** Ids of permission requests still waiting, oldest first. */
  pending = $state<number[]>([])
  /** Request ids this window answered, so "answered elsewhere" can be told from "answered here". */
  readonly mine = new Set<string>()
  private waiting = new Set<number>()
  private conn: ChatConnection
  private users = 0
  private gen = 0 // goes up on shut(), so a release from before it does nothing
  private unwake: (() => void) | null = null

  constructor(
    createSocket: () => SocketLike,
    private onInfo: (info: unknown) => void = () => {},
  ) {
    this.conn = new ChatConnection({
      createSocket,
      onWelcome: (w) => this.welcome(w),
      onSet: (id, m) => this.set(id, m),
      onAppend: (id, text) => this.append(id, text),
      onState: (s) => (this.wire = s),
    })
  }

  get pos() {
    return this.conn.pos
  }

  /**
   * The socket is open while anything holds it: the chat view, or a permission prompt on the board.
   * Returns the release for this hold; it runs once, and not at all after shut().
   */
  open(): () => void {
    if (this.users++ === 0) this.start()
    const gen = this.gen
    let done = false
    return () => {
      if (done || gen !== this.gen) return
      done = true
      this.close()
    }
  }

  /** Lets go of one hold; the socket closes with the last one. */
  close() {
    if (this.users > 0 && --this.users > 0) return
    this.stop()
  }

  /** Closes the socket whatever holds it, for a chat that stopped or was removed. */
  shut() {
    this.gen++
    this.users = 0
    this.stop()
  }

  private start() {
    this.conn.start()
    this.unwake ??= onWake(() => this.conn.wake())
  }

  private stop() {
    this.conn.stop()
    this.unwake?.()
    this.unwake = null
  }

  private welcome(w: Welcome) {
    if (!w.reset) return
    this.msgs.clear()
    this.waiting.clear()
    this.noAsk = false
    let top = -1
    for (const m of w.messages) {
      this.msgs.set(m.id, m)
      if (m.id > top) top = m.id
      this.track(m)
    }
    this.count = top + 1
    this.pending = [...this.waiting].sort((a, b) => a - b)
    if (w.info) this.onInfo(w.info)
  }

  private set(id: number, m: ChatMessage) {
    this.msgs.set(id, m)
    if (id >= this.count) this.count = id + 1
    if (this.track(m)) this.pending = [...this.waiting].sort((a, b) => a - b)
  }

  private append(id: number, text: string) {
    const m = this.msgs.get(id)
    if (m) this.msgs.set(id, { ...m, text: (m.text ?? '') + text })
  }

  /** Keeps the set of waiting requests; true when it changed. */
  private track(m: ChatMessage): boolean {
    if (m.kind === 'notice' && /did not switch to asking/.test(m.text ?? '')) this.noAsk = true
    if (m.kind !== 'permission') return false
    return m.state === 'pending' ? !this.waiting.has(m.id) && !!this.waiting.add(m.id) : this.waiting.delete(m.id)
  }
}
