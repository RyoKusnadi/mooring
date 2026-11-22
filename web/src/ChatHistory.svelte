<script lang="ts">
  // The "Chat history" dialog: searches every saved chat in every project.
  // Up and Down in the search box move between the results; Enter opens the one in focus.
  import { onDestroy, onMount, tick } from 'svelte'
  import SshModal from './SshModal.svelte'
  import { actions } from './lib/actions'
  import { byProject, pieces, searchChats, type ChatHit } from './lib/chatsearch'
  import { chats } from './lib/chatstore.svelte'
  import { relativeTime } from './lib/historyapi'
  import { app } from './lib/state.svelte'

  let { onClose }: { onClose: () => void } = $props()

  let q = $state('')
  let hits = $state<ChatHit[]>([])
  let loading = $state(true)
  let error = $state('')
  let box: HTMLElement | undefined = $state()
  let timer: ReturnType<typeof setTimeout> | undefined
  let ctl: AbortController | undefined

  const groups = $derived(
    byProject(hits)
      .map((g) => ({ ...g, name: app.projects.find((p) => p.id === g.projectId)?.name ?? '' }))
      .filter((g) => g.name),
  )
  const agentName = (id: string) => chats.agents.find((a) => a.id === id)?.name ?? id

  async function run(text: string) {
    ctl?.abort()
    const mine = (ctl = new AbortController())
    loading = true
    try {
      const found = await searchChats(text, undefined, mine.signal)
      if (mine.signal.aborted) return
      hits = found
      error = ''
    } catch (e) {
      if (mine.signal.aborted) return
      hits = []
      error = e instanceof Error ? e.message : String(e)
    }
    loading = false
  }

  function typed() {
    clearTimeout(timer)
    timer = setTimeout(() => void run(q), 200)
  }

  onMount(() => void run(''))
  onDestroy(() => {
    clearTimeout(timer)
    ctl?.abort()
  })

  const rows = () => [...(box?.querySelectorAll<HTMLButtonElement>('button.row') ?? [])]

  function open(h: ChatHit) {
    if (h.projectId !== app.currentId) app.select(h.projectId)
    actions.openChat(h.chatId)
    onClose()
  }

  function key(e: KeyboardEvent) {
    const list = rows()
    const at = list.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (!list.length) return
      e.preventDefault()
      const next = e.key === 'ArrowDown' ? at + 1 : at - 1
      if (next < 0) void tick().then(() => document.getElementById('chat-history-q')?.focus())
      else list[Math.min(next, list.length - 1)].focus()
    } else if (e.key === 'Enter' && at < 0 && (e.target as HTMLElement).id === 'chat-history-q') {
      e.preventDefault()
      list[0]?.click()
    }
  }
</script>

<SshModal title="Chat history" wide {onClose}>
  <div bind:this={box} onkeydown={key} role="presentation">
    <label class="label" for="chat-history-q">Search all chats in every project</label>
    <input id="chat-history-q" class="input" type="search" data-first autocomplete="off" spellcheck="false" maxlength="200" placeholder="Words from a chat" bind:value={q} oninput={typed} />
    <div class="results" aria-busy={loading}>
      {#if error}
        <p class="error" role="alert">{error}</p>
      {:else if loading && hits.length === 0}
        <p class="muted" role="status">Searching…</p>
      {:else if groups.length === 0}
        <p class="muted" role="status">{q.trim() ? 'No chat has all of these words.' : 'No chats are saved yet. Start one from the + menu in a pane.'}</p>
      {:else}
        <p class="muted" role="status">{q.trim() ? `${hits.length} ${hits.length === 1 ? 'chat' : 'chats'} found` : 'Newest chats'}</p>
        {#each groups as g (g.projectId)}
          <section aria-label={g.name}>
            <h3>{g.name}</h3>
            <ul>
              {#each g.hits as h (h.chatId)}
                <li>
                  <button type="button" class="row" onclick={() => open(h)}>
                    <span class="top"><span class="title">{h.title || 'New chat'}</span><span class="meta">{agentName(h.agent)} · {relativeTime(h.updatedAt / 1000)}</span></span>
                    {#if h.snippet}<span class="snip">{#each pieces(h.snippet) as p}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>{/if}
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/each}
      {/if}
    </div>
  </div>
</SshModal>

<style>
  .label {
    display: block;
    margin-bottom: var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .input {
    width: 100%;
  }
  .results {
    margin-top: var(--s-3);
    max-height: 55dvh;
    overflow-y: auto;
  }
  .muted,
  .error {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-s);
  }
  .muted {
    color: var(--muted);
  }
  .error {
    color: var(--danger);
  }
  h3 {
    margin: var(--s-3) 0 var(--s-1);
    font-size: var(--fs-s);
    font-weight: 600;
    color: var(--muted);
    overflow-wrap: anywhere;
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .row {
    display: grid;
    gap: 2px;
    width: 100%;
    min-height: var(--control-h);
    padding: var(--s-1) var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .row:hover {
    background: var(--hover);
  }
  .top {
    display: flex;
    gap: var(--s-3);
    justify-content: space-between;
    min-width: 0;
  }
  .title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    flex: none;
    color: var(--muted);
    font-size: var(--fs-xs);
  }
  .snip {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  mark {
    background: var(--selected);
    color: var(--fg);
    border-radius: var(--r-s);
  }
  @media (pointer: coarse) {
    .row {
      min-height: 44px;
    }
    .top {
      flex-direction: column;
      gap: 0;
    }
  }
</style>
