<script lang="ts">
  // A group tab: 2 or 3 chats of one task side by side, each a normal chat view with its own composer.
  import Chat from './Chat.svelte'
  import { actions } from './lib/actions'
  import { chats } from './lib/chatstore.svelte'
  import { groupTitle, leaveGroup, membersOf } from './lib/chatgroups'
  import { app } from './lib/state.svelte'

  let { id }: { id: string } = $props()

  const members = $derived(membersOf(chats.list, id))
  const title = $derived(groupTitle(members))
  let error = $state('')
  const run = <T,>(n: string, a: unknown) => app.run<T>(n, a)

  let cols: HTMLElement | undefined = $state()
  async function leave(chat: string) {
    error = ''
    try {
      await leaveGroup(run, chat)
      // The button is gone: put focus on the Leave button of a chat still in the group.
      queueMicrotask(() => cols?.querySelector<HTMLElement>('[data-leave]')?.focus())
    } catch (e) {
      error = 'Could not leave the group. ' + (e instanceof Error ? e.message : String(e))
    }
  }
  // Taking out all but one chat dissolves the group; the chats then open as tabs of their own.
  async function ungroup() {
    error = ''
    const all = members.map((m) => m.id)
    try {
      for (const m of all.slice(0, -1)) await leaveGroup(run, m)
    } catch (e) {
      error = 'Could not ungroup. ' + (e instanceof Error ? e.message : String(e))
      return
    }
    actions.closeTab(id)
    for (const m of all) actions.openChat(m)
  }
</script>

{#if !chats.known}
  <p class="note" role="status">Loading chats…</p>
{:else if members.length === 0}
  <div class="note">
    <p>This group no longer exists. Its chats are in the Chats list of the sidebar.</p>
    <button type="button" class="btn" onclick={() => actions.closeTab(id)}>Close this tab</button>
  </div>
{:else}
  <div class="group" data-group-view={id}>
    <header>
      <h2>{title}</h2>
      <button type="button" class="btn small" title="Take the chats out of the group and open each in its own tab" onclick={ungroup}>Ungroup</button>
    </header>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="cols" bind:this={cols} style:--n={members.length}>
      {#each members as m (m.id)}
        <section class="col" aria-label={`Chat ${m.title || 'New chat'}`}>
          <div class="bar">
            <span class="name">{chats.agents.find((a) => a.id === m.agent)?.name ?? m.agent}</span>
            <button type="button" class="btn ghost small" data-leave aria-label={`Leave group: ${m.title || 'New chat'}`} onclick={() => leave(m.id)}>Leave group</button>
          </div>
          <div class="body"><Chat id={m.id} /></div>
        </section>
      {/each}
    </div>
  </div>
{/if}

<style>
  .group {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    container-type: inline-size;
  }
  header,
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: 0 var(--s-2) 0 var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  header {
    min-height: var(--control-h);
  }
  h2 {
    flex: 1;
    min-width: 0;
    margin: 0;
    overflow: hidden;
    font-size: var(--fs-m);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cols {
    display: grid;
    flex: 1;
    grid-template-columns: repeat(var(--n), minmax(0, 1fr));
    min-height: 0;
  }
  .col {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .col + .col {
    border-left: 1px solid var(--border);
  }
  .bar {
    min-height: var(--control-h);
    background: var(--surface);
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .body {
    flex: 1;
    min-height: 0;
  }
  .note {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
  .error {
    margin: 0;
    padding: var(--s-2) var(--s-3);
  }
  @container (max-width: 720px) {
    .cols {
      grid-template-columns: minmax(0, 1fr);
      grid-auto-rows: minmax(420px, 1fr);
      overflow-y: auto;
    }
    .col + .col {
      border-top: 1px solid var(--border);
      border-left: 0;
    }
  }
</style>
