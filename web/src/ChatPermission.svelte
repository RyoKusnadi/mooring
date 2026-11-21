<script lang="ts">
  import { app } from './lib/state.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { inputSummary } from './lib/chatfmt'
  import type { ChatFeed } from './lib/chatfeed.svelte'

  let { id, feed, folder = '' }: { id: string; feed: ChatFeed; folder?: string } = $props()

  // The oldest request first; the first answer from any device wins.
  const msg = $derived(feed.pending.length ? feed.msgs.get(feed.pending[0]) : undefined)
  const more = $derived(feed.pending.length - 1)
  const command = $derived(inputSummary(msg?.rawInput))
  let busy = $state(false)
  let note = $state('')

  async function choose(optionId: string) {
    if (!msg?.requestId) return
    busy = true
    note = ''
    try {
      const r = await chats.answer((n, a) => app.run(n, a), id, msg.requestId, optionId)
      if (!r.accepted) {
        const name = msg.options?.find((o) => o.id === r.chosen)?.name ?? r.chosen
        note = `Another device answered first: ${name}.`
      }
    } catch (e) {
      note = e instanceof Error ? e.message : 'Could not send the answer.'
    } finally {
      busy = false
    }
  }
</script>

{#if msg}
  <section class="ask" role="alert" aria-label="Permission request">
    <div class="req">
      <p class="head"><span class="dot" aria-hidden="true"></span><strong>Needs permission</strong>{#if more > 0}<span class="more">{more} more waiting</span>{/if}</p>
      {#if !command || command !== msg.title}<p class="what">{msg.title}</p>{/if}
      {#if command}<code class="cmd" aria-label="Command" title={command}>{command}</code>{/if}
      {#if folder}<p class="where" title={folder}>in <code>{folder}</code></p>{/if}
    </div>
    <div class="options">
      {#each msg.options ?? [] as o (o.id)}
        <button type="button" class="btn" class:primary={o.kind === 'allow_once'} disabled={busy} onclick={() => choose(o.id)}>{o.name}</button>
      {/each}
    </div>
    {#if note}<p class="note" role="status">{note}</p>{/if}
  </section>
{/if}

<style>
  .ask {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--s-1) var(--s-4);
    padding: var(--s-2) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    border-top: 1px solid var(--border);
    background: var(--attention-bg);
    color: var(--attention);
  }
  .req {
    display: grid;
    min-width: 0;
  }
  .note {
    grid-column: 1 / -1;
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin: 0;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--attention);
  }
  .more {
    margin-left: auto;
    font-size: var(--fs-s);
  }
  .what,
  .note {
    margin: 0;
    overflow-wrap: anywhere;
  }
  code {
    padding: 0 var(--s-1);
    border-radius: var(--r-s);
    background: var(--bg);
    color: var(--fg);
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  .cmd {
    display: block;
    padding: var(--s-1) var(--s-2);
    overflow: hidden;
    font-size: var(--fs-code);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  @media (max-width: 900px) {
    .ask {
      grid-template-columns: 1fr;
    }
  }
  .where {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .options {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .options .btn:not(.primary) {
    background: var(--raised);
  }
</style>
