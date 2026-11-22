// Tells the server which chats the user is looking at, so "finished since you looked" stays true.
// Installed from App.svelte next to watchAttention.
import { app } from './state.svelte'
import { chats } from './chatstore.svelte'
import { seenToSend, visibleChats } from './seen'

export function watchSeen(): () => void {
  return $effect.root(() => {
    let shown = $state(!document.hidden)
    const onVisibility = () => (shown = !document.hidden)
    document.addEventListener('visibilitychange', onVisibility)

    let before = new Set<string>()
    const inflight = new Set<string>()
    // A failed send is tried once more; `retry` wakes the effect.
    const retried = new Set<string>()
    let retry = $state(0)
    $effect(() => {
      void retry
      const ids = shown ? visibleChats(app.layout) : []
      const send = seenToSend(
        before,
        ids,
        (id) => !!chats.list[id],
        (id) => !!chats.list[id]?.unseen && !inflight.has(id),
      )
      // A chat still unknown stays out of the set, so it is sent once the list has it.
      before = new Set(ids.filter((id) => chats.list[id]))
      for (const id of send) {
        inflight.add(id)
        app
          .run('chat.seen', { id })
          .then(() => retried.delete(id))
          .catch(() => {
            if (retried.has(id)) return
            retried.add(id)
            before.delete(id)
            retry++
          })
          .finally(() => inflight.delete(id))
      }
    })
    return () => document.removeEventListener('visibilitychange', onVisibility)
  })
}
