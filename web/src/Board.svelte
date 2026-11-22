<script lang="ts">
  // The status board: every chat in every project, grouped by what it needs from the user.
  import BoardRow from './BoardRow.svelte'
  import { actions } from './lib/actions'
  import { tick } from 'svelte'
  import { boardGroups, onBoard, type GroupKey } from './lib/board'
  import { GROUP_TITLE } from './lib/boardfmt'
  import { chats } from './lib/chatstore.svelte'
  import { git } from './lib/gitstore.svelte'
  import { app } from './lib/state.svelte'

  let { onNavigate }: { onNavigate: () => void } = $props()

  const groups = $derived(boardGroups(onBoard(Object.values(chats.list), app.projects.map((p) => p.id))))
  const projectName = (id: string) => app.projects.find((p) => p.id === id)?.name ?? ''
  const agentName = (id: string) => chats.agents.find((a) => a.id === id)?.name ?? id

  // Ages move on without a chat changing.
  let now = $state(Date.now())
  $effect(() => {
    const t = setInterval(() => (now = Date.now()), 30_000)
    return () => clearInterval(t)
  })

  // A row that changes group is rebuilt, which would drop keyboard focus to the page. The row
  // that had focus is found again in its new group, and a screen reader hears where it went.
  let said = $state('')
  let focused = ''
  const placed = new Map<string, GroupKey>()
  $effect(() => {
    const now = new Map<string, GroupKey>()
    for (const g of groups) for (const c of g.chats) now.set(c.id, g.key)
    const id = focused
    const before = id ? placed.get(id) : undefined
    const after = id ? now.get(id) : undefined
    placed.clear()
    for (const [k, v] of now) placed.set(k, v)
    if (!id || !after || before === after) return
    said = `${chats.list[id]?.title || 'The chat'} moved to ${GROUP_TITLE[after]}.`
    void tick().then(() => {
      if (document.activeElement === document.body) document.querySelector<HTMLElement>(`#board [data-chat="${id}"] .open`)?.focus()
    })
  })
  const track = (e: FocusEvent) => (focused = (e.target as HTMLElement).closest<HTMLElement>('[data-chat]')?.dataset.chat ?? '')

  // The status bar link opens the board and puts focus on its first waiting chat, or its first heading.
  $effect(() => {
    if (!git.focusBoard || !chats.known) return
    git.focusBoard = false
    void tick().then(() => document.querySelector<HTMLElement>('#board section:first-of-type .open, #board h3')?.focus())
  })

  function open(id: string, projectId: string) {
    if (projectId !== app.currentId) app.select(projectId)
    actions.openChat(id)
    onNavigate()
  }
</script>

<div id="board" class="board" aria-label="Status board" role="region" onfocusin={track}>
  <p class="sr" role="status">{said}</p>
  {#if !chats.known}
    <p class="hint" role="status">Loading chats…</p>
  {:else if groups.length === 0}
    <p class="hint">No chats yet. Start one with New chat in a pane, and it shows here with what it needs from you.</p>
  {:else}
    {#each groups as g (g.key)}
      <section aria-labelledby={`board-${g.key}`}>
        <h3 id={`board-${g.key}`} tabindex="-1">{GROUP_TITLE[g.key]} <span class="n">{g.chats.length + g.hidden}</span></h3>
        <ul>
          {#each g.chats as c (c.id)}
            <BoardRow chat={c} group={g.key} project={projectName(c.projectId)} agent={agentName(c.agent)} {now} onOpen={() => open(c.id, c.projectId)} />
          {/each}
        </ul>
        {#if g.hidden > 0}<p class="more">{g.hidden} older idle {g.hidden === 1 ? 'chat is' : 'chats are'} not shown. Find them with Chat history.</p>{/if}
      </section>
    {/each}
  {/if}
</div>

<style>
  .board {
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
  }
  section {
    padding-bottom: var(--s-2);
  }
  h3 {
    margin: 0;
    padding: var(--s-3) var(--s-3) var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
    font-weight: 600;
  }
  .n {
    font-weight: 400;
  }
  ul {
    margin: 0;
    padding: 0 var(--s-2);
    list-style: none;
  }
  .hint,
  .more {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
