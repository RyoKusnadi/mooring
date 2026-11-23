<script lang="ts">
  // Above the file list: what Changes shows. Uncommitted work, or what one chat's agent changed in its last turn.
  import { chats } from './lib/chatstore.svelte'
  import { findTab } from './lib/layout'
  import { app } from './lib/state.svelte'

  let { scope = $bindable('now'), chat = $bindable('') }: { scope: 'now' | 'turn'; chat: string } = $props()

  const list = $derived(chats.inProject(app.currentId))
  const title = (id: string) => chats.list[id]?.title || 'New chat'
  // The chat of the focused tab, else the one picked before, else the newest.
  const focused = $derived.by(() => {
    const l = app.layout
    const t = l ? findTab(l, l.focus) : undefined
    return t?.kind === 'chat' && list.some((c) => c.id === t.id) ? t.id : ''
  })
  $effect(() => {
    if (scope !== 'turn') return
    if (!chat || !list.some((c) => c.id === chat)) chat = focused || list[0]?.id || ''
  })
  function showTurn() {
    if (focused) chat = focused
    scope = 'turn'
  }
</script>

{#if list.length > 0 || scope === 'turn'}
<div class="scope" role="group" aria-label="What to show">
  <button type="button" class="btn small" aria-pressed={scope === 'now'} onclick={() => (scope = 'now')}>Uncommitted</button>
  <button type="button" class="btn small" aria-pressed={scope === 'turn'} disabled={list.length === 0} onclick={showTurn}>
    {scope === 'turn' && chat ? `Last turn of ${title(chat)}` : 'Last turn'}
  </button>
  {#if scope === 'turn' && list.length > 1}
    <select class="input" aria-label="Chat for the last turn" bind:value={chat}>
      {#each list as c (c.id)}<option value={c.id}>{title(c.id)}</option>{/each}
    </select>
  {/if}
</div>
{/if}

<style>
  .scope {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .btn[aria-pressed='true'] {
    background: var(--selected);
    font-weight: var(--fw-semibold);
  }
  .btn {
    max-width: 36ch;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  select {
    min-width: 0;
    max-width: 28ch;
  }
</style>
