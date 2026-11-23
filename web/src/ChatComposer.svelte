<script lang="ts">
  import { composing, modLabel } from './lib/keys'
  import { untrack } from 'svelte'
  import ChatChips from './ChatChips.svelte'
  import ChatImageButton from './ChatImageButton.svelte'
  import ChatNotes from './ChatNotes.svelte'
  import ChatModes from './ChatModes.svelte'
  import ChatQueue from './ChatQueue.svelte'
  import { app } from './lib/state.svelte'
  import { composerHint, composerWhy } from './lib/chatfmt'
  import { attach, wire, type Attachment } from './lib/attachstore.svelte'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { loadQueue, saveQueue } from './lib/queuestore'
  import { ImagePicker } from './lib/imagepick.svelte'
  import { noImagesNote } from './lib/images'

  let { id, info, agent, onSent, onStopped }: { id: string; info: ChatInfo; agent: string; onSent: () => void; onStopped: () => void } = $props()

  let text = $state(untrack(() => chats.drafts.get(id)) ?? '')
  let error = $state('')
  let sending = $state(false)
  let box: HTMLTextAreaElement | undefined = $state()

  // Whether the agent takes images is known once it has started (and is saved with the chat).
  const known = $derived(info.caps?.initialized === true)
  // Until the agent has started its abilities are unknown: attaching is allowed, and a send the agent cannot take is refused with the reason.
  const takes = $derived(!known || info.caps?.image === true)
  const picker = new ImagePicker({ id: () => id, takes: () => takes, done: () => box?.focus() })

  // Messages written while the agent replies. Each goes out when the reply ends.
  // The text is also kept in this browser, so a reload brings it back (without attachments or images).
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
  const noteText = $derived(known && !takes ? noImagesNote(agent) : '')
  const hint = $derived(composerHint(info.state, agent, picker.busy, sending && app.status !== 'live'))

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
  const pasteHint = $derived(ready && takes && !chips.length && !text.trim() ? `You can also paste an image (${modLabel}V) or drop one on the message box.` : '')
  const note = $derived([hint, noteText || pasteHint].filter(Boolean).join(' '))

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
      picker.clear()
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
    for (const c of q.chips) attach.add(id, c)
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
    <ChatQueue {queue} onTakeBack={takeBack} />
    {#if why}<p class="why" id={'why-' + id}>{why}</p>{/if}
    <div class="field" class:invalid={!!picker.error} ondragenter={picker.dragEnter} ondragover={picker.dragOver} ondragleave={picker.dragLeave} ondrop={picker.drop} role="presentation">
      <ChatChips chips={chips} chatId={id} onRemove={(i) => (attach.remove(id, i), picker.clear())} />
      <textarea
        id={'msg-' + id}
        class="input"
        rows="1"
        placeholder={ready || running ? 'Ask or tell it what to do' : ''}
        aria-describedby={[why && 'why-' + id, picker.error && 'err-' + id, note && 'note-' + id].filter(Boolean).join(' ') || undefined}
        aria-invalid={picker.error ? 'true' : undefined}
        onpaste={picker.paste}
        bind:value={text}
        bind:this={box}
        onkeydown={keys}
      ></textarea>
      <div class="tools">
        <span class="attach"><ChatImageButton disabled={!takes} describedby={noteText ? 'note-' + id : undefined} onPick={(f) => picker.add(f)} /></span>
        <ChatModes {id} {info} {agent} />
        <span class="grow"></span>
        {#if running}<button type="button" class="btn" onclick={stop}>Stop reply</button>{/if}
        <button type="submit" class="btn primary" disabled={(!ready && !running) || !text.trim() || sending || picker.busy > 0}>{running ? 'Send after reply' : 'Send'}</button>
      </div>
      <div class="drop" aria-hidden="true" class:on={picker.drag}>Drop images to attach</div>
    </div>
    <ChatNotes {id} error={error || picker.error} {note} urgent={picker.refused && !!noteText} />
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
    font-weight: var(--fw-semibold);
  }
  .field {
    container-type: inline-size;
    position: relative;
    display: grid;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2) var(--s-2);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-m);
    background: var(--raised);
  }
  /* One 2px ring: the border plus a 1px shadow in the same colour. Forced-colors mode drops the
     shadow but draws the transparent outline, so the ring stays visible there. */
  .field:focus-within {
    border-color: var(--focus-ring);
    box-shadow: 0 0 0 1px var(--focus-ring);
    outline: 1px solid transparent;
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
  .field.invalid {
    border-color: var(--danger);
  }
  .field.invalid:focus-within {
    border-color: var(--focus-ring);
  }
  .drop {
    position: absolute;
    inset: -1px;
    display: none;
    place-items: center;
    border: 2px dashed var(--accent);
    border-radius: var(--r-m);
    background: var(--selected);
    font-weight: var(--fw-medium);
    pointer-events: none;
  }
  .drop.on {
    display: grid;
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
  /* A narrow box (a phone, or a small split pane): Model and Mode share the first row and the buttons get the next, so nothing shrinks. */
  @container (max-width: 520px) {
    .tools :global(.pick) {
      flex: 1 1 40%;
      min-width: 120px;
    }
    .grow {
      flex: 1 1 100%;
      order: 1;
      height: 0;
      margin: calc(var(--s-1) * -1) 0;
    }
    .attach {
      order: 2;
    }
    .tools .btn {
      flex: 1 1 0;
      order: 3;
      white-space: nowrap;
    }
  }
</style>
