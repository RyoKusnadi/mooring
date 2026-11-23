<script lang="ts">
  // Sends one finished reply to another chat of the project. Loads when the user asks for it.
  // Nothing is sent until Send is pressed.
  import { onMount, tick } from 'svelte'
  import { actions } from './lib/actions'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { busyReason, canTake, startNote } from './lib/chatrelay'
  import { app } from './lib/state.svelte'

  let { chatId, msgId, onclose }: { chatId: string; msgId: number; onclose: (refocus?: boolean) => void } = $props()

  const projectId = $derived(chats.list[chatId]?.projectId)
  const others = $derived(
    Object.values(chats.list)
      .filter((c: ChatInfo) => c.projectId === projectId && c.id !== chatId)
      .sort((a, b) => b.updatedAt - a.updatedAt),
  )
  const agentName = (id: string) => chats.agents.find((a) => a.id === id)?.name ?? id
  const titleOf = (c: ChatInfo) => c.title || 'New chat'

  let target = $state('')
  let note = $state('')
  let busy = $state(false)
  let error = $state('')
  let sent = $state<{ id: string; label: string } | null>(null)
  let openButton: HTMLButtonElement | undefined = $state()
  let panel: HTMLElement | undefined = $state()
  const uid = $props.id()

  // A target that began answering while the panel was open can no longer be sent to.
  const ready = $derived(!!target && canTake(chats.list[target]?.state ?? ''))
  const lost = $derived(target && !ready && !busy ? busyReason(chats.list[target]?.state ?? '') : '')
  const none = $derived(others.length > 0 && others.every((c) => !canTake(c.state)))
  let starting = $state(false)

  onMount(async () => {
    await tick()
    panel?.querySelector<HTMLElement>('input:not(:disabled), button:not(:disabled)')?.focus()
  })

  async function send() {
    const to = chats.list[target]
    if (!to || busy) return
    busy = true
    starting = to.state === 'stopped'
    error = ''
    try {
      await app.run('chat.relay', { id: target, from: chatId, msgIdx: msgId, text: note.trim() })
      sent = { id: target, label: `${agentName(to.agent)}, ${titleOf(to)}` }
      await tick()
      openButton?.focus()
    } catch (e) {
      error = `${agentName(to.agent)}: ${e instanceof Error ? e.message : 'Could not send the reply.'}`
    } finally {
      busy = false
      starting = false
    }
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="relay" role="region" aria-label="Send a reply to another chat" bind:this={panel} onkeydown={(e) => e.key === 'Escape' && (e.stopPropagation(), onclose())}>
  {#if sent}
    <p role="status" class="done">Sent to {sent.label}.</p>
    <div class="row">
      <button type="button" class="btn primary" bind:this={openButton} onclick={() => actions.openChat(sent!.id)}>Open that chat</button>
      <button type="button" class="btn" onclick={() => onclose()}>Close</button>
    </div>
  {:else}
    {#if others.length === 0}
      <p class="help">There is no other chat in this project. Choose New chat to start one, then send the reply from here.</p>
      <div class="row">
        <button type="button" class="btn" onclick={() => (actions.newChat(), onclose(false))}>New chat</button>
        <button type="button" class="btn ghost" onclick={() => onclose()}>Close</button>
      </div>
    {:else}
      <fieldset>
      <legend>Send this reply to</legend>
      <ul>
        {#each others as c (c.id)}
          {@const why = busyReason(c.state)}
          <li>
            <label class="opt" class:off={!!why}>
              <input type="radio" name="{uid}-to" value={c.id} bind:group={target} disabled={!!why} />
              <span class="who">{agentName(c.agent)}</span>
              <span class="title">{titleOf(c)}</span>
              {#if why}<span class="why">({why})</span>{:else if startNote(c)}<span class="why">(stopped) {startNote(c)}</span>{/if}
            </label>
          </li>
        {/each}
      </ul>
      </fieldset>
      {#if target}
        <label class="field" for="{uid}-n">Add a note (optional)</label>
        <textarea id="{uid}-n" class="input note" rows="2" bind:value={note}></textarea>
      {/if}
      <div class="row">
        <button type="button" class="btn primary" disabled={!ready || busy} onclick={send}>{busy ? (starting ? 'Starting the chat…' : 'Sending…') : 'Send'}</button>
        <button type="button" class="btn ghost" onclick={() => onclose()}>Cancel</button>
      </div>
      {#if !ready && !busy && !lost && !none}<p class="help">Choose a chat to send to.</p>{/if}
      {#if lost}<p class="help" role="status">{agentName(chats.list[target]?.agent ?? '')} is {lost} now. Pick another chat.</p>{/if}
      {#if none}<p class="help">No other chat can take a reply right now. Each one says why.</p>{/if}
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    {/if}
  {/if}
</div>

<style>
  .relay {
    display: grid;
    gap: var(--s-2);
    max-width: 420px;
    margin-top: var(--s-1);
    padding: var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background: var(--raised);
  }
  fieldset {
    min-width: 0;
    margin: 0;
    padding: 0;
    border: 0;
  }
  legend {
    padding: 0;
    font-weight: var(--fw-semibold);
  }
  .done {
    margin: 0;
    font-weight: var(--fw-semibold);
  }
  .field {
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  ul {
    display: grid;
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .opt {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-2);
    min-height: var(--control-h);
    overflow-wrap: anywhere;
  }
  .opt.off {
    color: var(--muted);
  }
  .who {
    font-weight: var(--fw-semibold);
  }
  .note {
    min-height: 60px;
    padding: var(--s-2);
    resize: vertical;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
</style>
