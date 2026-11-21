<script lang="ts">
  // Several chats are open: ask which one gets the lines, then show it.
  import { onMount } from 'svelte'
  import { attach } from './lib/attachstore.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { app } from './lib/state.svelte'
  import { relativeTime } from './lib/historyapi'

  let dialog: HTMLDialogElement | undefined = $state()
  const ask = $derived(attach.asking)
  const agentName = (id: string) => chats.agents.find((a) => a.id === id)?.name ?? id

  onMount(() => dialog?.showModal())

  function choose(id: string) {
    const err = attach.asking?.send(id) ?? ''
    attach.asking = null
    if (err) app.notice = err
  }
</script>

{#if ask}
  <dialog class="dlg" bind:this={dialog} aria-labelledby="pick-h" onclose={() => (attach.asking = null)}>
    <h2 id="pick-h">Send to which chat?</h2>
    <ul>
      {#each ask.ids as id (id)}
        <li>
          <button type="button" class="btn" onclick={() => choose(id)}>
            <span class="col">
              <span class="t">{chats.list[id]?.title || 'New chat'}</span>
              {#if chats.list[id]?.lastLine}<span class="l">{chats.list[id]?.lastLine}</span>{/if}
            </span>
            <span class="a">{agentName(chats.list[id]?.agent ?? '')} · {relativeTime((chats.list[id]?.updatedAt ?? 0) / 1000)}</span>
          </button>
        </li>
      {/each}
    </ul>
    <footer><button type="button" class="btn" onclick={() => dialog?.close()}>Cancel</button></footer>
  </dialog>
{/if}

<style>
  h2 {
    margin: 0;
    padding: var(--s-3) var(--s-4) var(--s-2);
    font-size: var(--fs-l);
  }
  ul {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0 var(--s-4) var(--s-3);
    list-style: none;
  }
  .btn {
    display: flex;
    width: 100%;
    justify-content: space-between;
    gap: var(--s-3);
    align-items: center;
    height: auto;
    padding-block: var(--s-1);
    text-align: left;
  }
  .col {
    display: grid;
    min-width: 0;
  }
  .l {
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .t {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .a {
    flex: none;
    white-space: nowrap;
    color: var(--muted);
  }
</style>
