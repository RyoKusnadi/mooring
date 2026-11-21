<script lang="ts">
  import AttentionMark from './AttentionMark.svelte'
  import { actions } from './lib/actions'
  import { chats } from './lib/chatstore.svelte'
  import { attentionLabel } from './lib/chatfmt'
  import { app } from './lib/state.svelte'
  import { relativeTime } from './lib/historyapi'

  let { onNavigate }: { onNavigate: () => void } = $props()

  const STATE: Record<string, string> = { starting: 'Starting', idle: '', running: 'Replying', needs_login: 'Not signed in', stopped: 'Stopped' }
  const all = $derived(chats.inProject(app.currentId))
  let find = $state('')
  const saved = $derived(find.trim() ? all.filter((c) => (c.title || 'New chat').toLowerCase().includes(find.trim().toLowerCase())) : all)
  let confirming = $state('')
  let renaming = $state('')
  let draft = $state('')
  const agentName = (id: string) => chats.agents.find((a) => a.id === id)?.name ?? id
  function startRename(id: string, title: string) {
    renaming = id
    draft = title
  }
  async function saveRename() {
    const id = renaming
    renaming = ''
    if (!id || !draft.trim() || draft.trim() === chats.list[id]?.title) return
    try {
      await app.run('chat.rename', { id, title: draft.trim() })
    } catch (e) {
      app.notice = 'Could not rename the chat. ' + (e instanceof Error ? e.message : String(e))
    }
  }
</script>

{#if app.ready && app.current}
  <section aria-label={`Chats in ${app.current.name}`}>
    <h2>Chats</h2>
    {#if !chats.known}
      <p class="hint" role="status">Loading chats…</p>
    {:else if all.length === 0}
      <p class="hint">No chats in this project yet. Start one from the + menu in a pane.</p>
    {:else}
      {#if all.length > 5}
        <input class="input find" type="search" aria-label="Filter chats" placeholder="Filter chats" autocomplete="off" bind:value={find} />
      {/if}
      {#if saved.length === 0}
        <p class="hint">No chats match "{find}". <button type="button" class="btn small" onclick={() => (find = '')}>Clear</button></p>
      {/if}
      <ul>
        {#each saved as c (c.id)}
          <li>
            {#if confirming === c.id}
              <span class="ask">Delete this chat?</span>
              <button type="button" class="btn danger small" onclick={() => (actions.deleteChat(c.id), (confirming = ''))}>Delete</button>
              <button type="button" class="btn small" onclick={() => (confirming = '')}>Keep</button>
            {:else if renaming === c.id}
              <input class="input rename" aria-label="Chat title" maxlength="120" bind:value={draft} onblur={saveRename} onkeydown={(e) => (e.key === 'Enter' ? saveRename() : e.key === 'Escape' && ((renaming = ''), e.stopPropagation()))} {@attach (n: HTMLInputElement) => (n.focus(), n.select())} />
            {:else}
              <button type="button" class="open" title={c.title} onclick={() => (actions.openChat(c.id), onNavigate())}>
                <span class="text"><span class="title">{c.title || 'New chat'}</span>{#if c.lastLine}<span class="last">{c.lastLine}</span>{/if}<span class="when">{agentName(c.agent)} · {relativeTime(c.updatedAt / 1000)}</span></span>
                {#if c.attention}
                  <AttentionMark label={attentionLabel(c)} />
                {:else if STATE[c.state]}
                  <span class="state">{STATE[c.state]}</span>
                {/if}
              </button>
              <button type="button" class="del" aria-label={`Rename chat ${c.title || 'New chat'}`} title="Rename" onclick={() => startRename(c.id, c.title)}>✎</button>
              <button type="button" class="del" aria-label={`Delete chat ${c.title || 'New chat'}`} onclick={() => (confirming = c.id)}>×</button>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </section>
{/if}

<style>
  section {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    min-height: 0;
    border-top: 1px solid var(--border);
  }
  h2 {
    margin: 0;
    padding: var(--s-3) var(--s-3) var(--s-1);
    font-size: var(--fs-s);
    font-weight: 600;
    color: var(--muted);
  }
  .hint {
    margin: 0;
    padding: var(--s-1) var(--s-3) var(--s-2);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  ul {
    margin: 0;
    padding: 0 var(--s-2) var(--s-2);
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    min-height: var(--control-h);
    border-radius: var(--r-s);
  }
  li:hover {
    background: var(--hover);
  }
  .open {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .text {
    display: grid;
    flex: 1;
    min-width: 0;
    padding: var(--s-1) 0;
  }
  .when {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-size: var(--fs-xs);
    line-height: 14px;
  }
  .last {
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .find,
  .rename {
    margin: 0 var(--s-2) var(--s-1);
    width: calc(100% - var(--s-4));
  }
  .rename {
    flex: 1;
    margin: 0;
  }
  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .state {
    flex: none;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .del {
    flex: none;
    width: var(--control-h);
    min-height: var(--control-h);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--muted);
    font: inherit;
    opacity: 0;
  }
  li:hover .del,
  li:focus-within .del {
    opacity: 1;
  }
  .del:hover {
    color: var(--fg);
  }
  .ask {
    flex: 1;
    padding: 0 var(--s-2);
  }
  .small {
    min-height: calc(var(--control-h) - 4px);
    padding: 0 var(--s-2);
  }
  @media (pointer: coarse) {
    .del {
      opacity: 1;
    }
  }
</style>
