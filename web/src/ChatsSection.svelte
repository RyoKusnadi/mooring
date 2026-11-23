<script lang="ts">
  import AttentionMark from './AttentionMark.svelte'
  import Icon from './Icon.svelte'
  import { actions } from './lib/actions'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { chatRows } from './lib/chatgroups'
  import { attentionLabel } from './lib/chatfmt'
  import { split } from './lib/sidebarsplit.svelte'
  import { app } from './lib/state.svelte'
  import { deleteQuestion } from './lib/chatfolders'
  import { relativeTime } from './lib/historyapi'

  let { onNavigate }: { onNavigate: () => void } = $props()

  const STATE: Record<string, string> = { starting: 'Starting', idle: '', running: 'Replying', needs_login: 'Not signed in', stopped: 'Stopped' }
  const all = $derived(chats.inProject(app.currentId))
  let find = $state('')
  const saved = $derived(find.trim() ? all.filter((c) => (c.title || 'New chat').toLowerCase().includes(find.trim().toLowerCase())) : all)
  const rows = $derived(chatRows(saved))
  // With the filter on, say how many of the group's chats match.
  const groupCount = (gid: string, shown: number) => {
    const total = all.filter((c) => c.groupId === gid).length
    return shown < total ? `${shown} of ${total} chats match` : `Group of ${total} chats`
  }
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
    <div class="head">
      <h2>
        <button type="button" class="fold" aria-expanded={split.open} onclick={() => split.setOpen(!split.open)}>
          <span class="chev" class:open={split.open}><Icon name="chevronRight" /></span>
          Chats{#if chats.known && all.length > 0}<span class="n">{all.length}</span>{/if}
          {#if !split.open && all.some((c) => c.attention)}<AttentionMark label="A chat needs you" />{/if}
        </button>
      </h2>
      <button type="button" class="btn ghost small" aria-label="Search chat history" title="Search every chat in every project" onclick={actions.openChatHistory}>History</button>
    </div>
    {#if !split.open}
      <!-- Closed: the header is all there is. -->
    {:else if !chats.known}
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
      {#snippet row(c: ChatInfo)}
        <li>
      {#if confirming === c.id}
        <span class="ask">{deleteQuestion(chats.list[c.id], agentName(c.agent))}</span>
        <button type="button" class="btn danger solid small" onclick={() => (actions.deleteChat(c.id), (confirming = ''))}>Delete</button>
        <button type="button" class="btn small" onclick={() => (confirming = '')} {@attach (n: HTMLButtonElement) => n.focus()}>Keep</button>
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
      {/snippet}
      <ul>
        {#each rows as r (('group' in r ? 'g' + r.group.id : r.chat.id))}
          {#if 'group' in r}
            <li class="group">
              <button type="button" class="open" title={r.group.title} onclick={() => (actions.openGroup(r.group.id), onNavigate())}>
                <span class="text"><span class="title">{r.group.title}</span><span class="when">{groupCount(r.group.id, r.group.members.length)}</span></span>
                {#if r.group.attention}<AttentionMark label="A chat in this group needs you" />{/if}
              </button>
              <ul class="members" aria-label={`Chats in ${r.group.title}`}>
                {#each r.group.members as m (m.id)}{@render row(m)}{/each}
              </ul>
            </li>
          {:else}
            {@render row(r.chat)}
          {/if}
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
  }
  .head {
    display: flex;
    flex: none;
    align-items: center;
    justify-content: space-between;
    padding: var(--s-1) var(--s-2) 0 var(--s-2);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .fold {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    min-height: var(--control-h);
    padding: 0 var(--s-1);
    border: 0;
    border-radius: var(--r-s);
    background: none;
    color: var(--muted);
    font: inherit;
    font-weight: var(--fw-semibold);
  }
  .fold:hover {
    background: var(--hover);
    color: var(--fg);
  }
  .chev {
    display: inline-flex;
  }
  .chev :global(svg) {
    transition: transform var(--ease);
  }
  .chev.open :global(svg) {
    transform: rotate(90deg);
  }
  .n {
    font-weight: var(--fw-regular);
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
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1);
    min-height: var(--control-h);
    border-radius: var(--r-s);
  }
  li:hover {
    background: var(--hover);
  }
  li.group {
    display: block;
  }
  li.group:hover {
    background: transparent;
  }
  li.group > .open {
    width: 100%;
    border-radius: var(--r-s);
  }
  li.group > .open:hover {
    background: var(--hover);
  }
  .members {
    margin: 0;
    padding: 0 0 0 var(--s-3);
    overflow: visible;
    border-left: 1px solid var(--border);
    margin-left: var(--s-3);
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
    line-height: var(--lh-xs);
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
    flex: 1 1 14ch;
    min-width: 0;
    padding: 0 var(--s-2);
  }
  .small {
    min-height: var(--control-h-s);
    padding: 0 var(--s-2);
  }
  @media (pointer: coarse) {
    .del {
      opacity: 1;
    }
    .small {
      min-height: 44px;
    }
  }
</style>
