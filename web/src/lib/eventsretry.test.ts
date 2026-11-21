import { describe, expect, it } from 'vitest'
import { EventsClient, type Clock, type SocketLike } from './events'

function socket(): SocketLike & { open(): void } {
  const s = { onopen: null, onmessage: null, onclose: null, onerror: null, send() {}, close() {}, open: () => s.onopen?.() } as SocketLike & { open(): void }
  return s
}

describe('retryNow', () => {
  it('connects at once instead of waiting for the backoff, and leaves a connection that is up alone', () => {
    const sockets: ReturnType<typeof socket>[] = []
    const waiting: (() => void)[] = []
    const clock: Clock = { setTimeout: (fn) => waiting.push(fn), clearTimeout: () => {} }
    const client = new EventsClient({
      createSocket: () => sockets[sockets.push(socket()) - 1],
      clock,
      onReset: () => {},
      onEvent: () => {},
      onStatus: () => {},
    })
    client.start()
    sockets[0].onclose?.()
    expect(sockets).toHaveLength(1)
    client.retryNow()
    expect(sockets).toHaveLength(2)
    sockets[1].onmessage?.({ data: JSON.stringify({ t: 'welcome', epoch: 'e', pos: 1, snapshot: {} }) })
    client.retryNow()
    expect(sockets).toHaveLength(2)
  })
})
