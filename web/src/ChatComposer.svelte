<script lang="ts">
  import { composing } from './lib/keys'
  import { untrack } from 'svelte'
  import ChatChips from './ChatChips.svelte'
  import ChatModes from './ChatModes.svelte'
  import { app } from './lib/state.svelte'
  import { composerWhy } from './lib/chatfmt'
  import { attach, wire, type Attachment } from './lib/attachstore.svelte'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { loadQueue, saveQueue } from './lib/queuestore'

  let { id, info, agent, onSent, onStopped }: { id: string; info: ChatInfo; agent: string; onSent: () => void; onStopped: () => void } = $props()

  let text = $state(untrack(() => chats.drafts.get(id)) ?? '')
  let error = $state('')
  let sending = $state(false)
  let box: HTMLTextAreaElement | undefined = $state()

  // Messages written while the agent replies. Each goes out when the reply ends.
  // The text is also kept in this browser, so a reload brings it back (without attachments).
  let queue = $state<{ text: string; chips: Attachment[] }[]>(untrack(() => chats.queued.get(id)) ?? untrack(() => loadQueue(id)).map((text) => ({ text, chips: [] })))
  $effect(() => {
    chats.queued.set(id, queue)
    saveQueue(
      id,
      queue.map((q) => q.text),
    )
  })

  const running = $derived(info.state === 'running')
  const ready = $derived(info.state === 'idle')
  const why = $derived(running ? '' : composerWhy(info.state))
  const hint = $derived(
    sending && app.status !== 'live'
      ? 'Waiting to send. The message goes out when Mooring reconnects.'
      : info.state === 'starting'
      ? `Starting ${agent}…`
      : info.state === 'running'
        ? `${agent} is replying. A message you send now goes out when the reply ends. Queued messages stay if the page reloads, but their attachments do not.`
        : info.state === 'idle'
          ? ''
          : info.state === 'needs_login'
            ? ''
            : `${agent} is not running. Resume the chat to send a message.`,
  )

  $effect(() => {
    chats.drafts.set(id, text)
  })

  // Terminal text sent to this chat lands in the draft.
  $effect(() => {
    const t = attach.texts[id] && attach.takeText(id)
    if (t) {
      text = text.trim() ? text.replace(/\s*$/, '\n\n') + t : t
      box?.focus()
    }
  })

  const chips = $derived(attach.list(id))

  // The box grows with the text, up to a few lines.
  $effect(() => {
    void text
    if (!box) return
    box.style.height = 'auto'
    box.style.height = Math.min(box.scrollHeight, 180) + 'px'
  })

  async function deliver(body: string, sentChips: Attachment[]): Promise<boolean> {
    sending = true
    error = ''
    try {
      await app.run('chat.prompt', { id, text: body, ...(sentChips.length ? { attachments: wire(sentChips) } : {}) })
      onSent()
      return true
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not send the message.'
      return false
    } finally {
      sending = false
    }
  }

  async function send() {
    const body = text.trim()
    if (!body || sending) return
    if (running) {
      queue = [...queue, { text: body, chips }]
      text = ''
      attach.clear(id)
      return
    }
    if (!ready) return
    if (await deliver(body, chips)) {
      text = ''
      attach.clear(id)
    }
    box?.focus()
  }

  // The reply ended in a limit, a failure or a stop: nothing queued goes out. It comes back to the box.
  const lastMsg = $derived(chats.feed(id).msgs.get(chats.feed(id).count - 1))
  const failed = $derived(ready && lastMsg?.kind === 'error')
  $effect(() => {
    if (queue.length === 0 || running || info.state === 'starting' || sending) return
    if (!failed && ready) return
    text = [...queue.map((q) => q.text), untrack(() => text).trim()].filter(Boolean).join('\n\n')
    queue = []
    error = 'Not sent. The reply did not finish, so your queued message is back in the box.'
  })

  // The reply ended: the first queued message goes out. A failed send keeps the rest queued and shows why.
  $effect(() => {
    if (!ready || failed || sending || error || queue.length === 0) return
    const [next, ...rest] = queue
    queue = rest
    void deliver(next.text, next.chips).then((ok) => {
      if (!ok) queue = [next, ...untrack(() => queue)]
    })
  })

  function takeBack(i: number) {
    const q = queue[i]
    queue = queue.filter((_, j) => j !== i)
    text = text.trim() ? text.replace(/\s*$/, '\n\n') + q.text : q.text
    box?.focus()
  }

  async function stop() {
    error = ''
    try {
      onStopped()
      // Stopping hands queued messages back to the box instead of sending them behind the user's back.
      if (queue.length) {
        text = [...queue.map((q) => q.text), text.trim()].filter(Boolean).join('\n\n')
        queue = []
      }
      await app.run('chat.cancel', { id })
      // The Stop button is replaced by Send, so put focus where typing continues.
      box?.focus()
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not stop the reply.'
    }
  }

  function keys(e: KeyboardEvent) {
    if (e.key !== 'Enter' || e.shiftKey || composing(e)) return
    if (matchMedia('(pointer: coarse)').matches) return
    e.preventDefault()
    void send()
  }
</script>

<form class="composer" onsubmit={(e) => (e.preventDefault(), send())}>
  <div class="col">
    <label for={'msg-' + id}>Message to {agent}</label>
    {#if queue.length}
      <ul class="queue" aria-label="Queued messages">
        {#each queue as q, i (i)}
          <li>
            <span class="qt">{q.text}</span>
            <span class="qs">Sends when the reply ends</span>
            <button type="button" class="btn small" onclick={() => takeBack(i)}>Take back<span class="sr">: {q.text.slice(0, 40)}</span></button>
          </li>
        {/each}
      </ul>
    {/if}
    {#if why}<p class="why" id={'why-' + id}>{why}</p>{/if}
    <div class="field">
      <ChatChips chips={chips} onRemove={(i) => attach.remove(id, i)} />
      <textarea
        id={'msg-' + id}
        class="input"
        rows="1"
        placeholder={ready || running ? 'Ask or tell it what to do' : ''}
        aria-describedby={why ? 'why-' + id : undefined}
        bind:value={text}
        bind:this={box}
        onkeydown={keys}
      ></textarea>
      <div class="tools">
        <ChatModes {id} {info} {agent} />
        <span class="grow"></span>
        {#if running}<button type="button" class="btn" onclick={stop}>Stop reply</button>{/if}
        <button type="submit" class="btn primary" disabled={(!ready && !running) || !text.trim() || sending}>{running ? 'Send after reply' : 'Send'}</button>
      </div>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{:else if hint}<p class="help" role="status">{hint}</p>{/if}
  </div>
</form>

<style>
  .composer {
    padding: var(--s-2) max(var(--s-4), calc((100% - var(--chat-col)) / 2)) calc(var(--s-2) + env(safe-area-inset-bottom, 0px));
    border-top: 1px solid var(--border);
    background: var(--surface);
  }
  .col {
    display: grid;
    gap: var(--s-1);
  }
  .why {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  label {
    color: var(--muted);
    font-size: var(--fs-s);
    font-weight: 600;
  }
  .field {
    display: grid;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2) var(--s-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-m);
    background: var(--bg);
  }
  .field:focus-within {
    border-color: var(--accent);
  }
  textarea {
    min-height: var(--control-h);
    max-height: 180px;
    padding: var(--s-1) 0;
    border: 0;
    background: transparent;
    resize: none;
    line-height: 1.5;
    outline: none;
  }
  .tools {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
  }
  .grow {
    flex: 1;
  }
  /* A phone: Model and Mode share the first row and the buttons get the next, so nothing shrinks. */
  @media (max-width: 900px) {
    .tools :global(.pick) {
      flex: 1 1 40%;
      min-width: 120px;
    }
    .grow {
      flex: 1 1 100%;
      height: 0;
      margin: calc(var(--s-1) * -1) 0;
    }
    .tools .btn {
      flex: 1 1 0;
      white-space: nowrap;
    }
  }
  .queue {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .queue li {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-m);
  }
  .qt {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .qs {
    flex: none;
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
