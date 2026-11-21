<script lang="ts">
  // History of a place: every commit on every branch, newest first, with a graph. The list is virtual and
  // reads the next page near its end. Selecting a commit shows its message and files below or beside it.
  import { onMount, tick, untrack } from 'svelte'
  import CommitDetail from './CommitDetail.svelte'
  import HistoryRow from './HistoryRow.svelte'
  import VirtualList from './VirtualList.svelte'
  import { git } from './lib/gitstore.svelte'
  import { History } from './lib/history.svelte'
  import type { SearchIn } from './lib/historyapi'
  import { app } from './lib/state.svelte'

  let { folder, commit = '' }: { folder: string; commit?: string } = $props()

  const MAX_LANES = 10
  const NEAR_END = 40
  const hist = new History(untrack(() => app.currentId), untrack(() => folder))
  let list: VirtualList | undefined = $state()
  let wrap: HTMLElement | undefined = $state()
  let selected = $state('')
  let active = $state(0)
  let now = $state(Date.now())
  let q = $state('')
  let mode = $state<SearchIn>('message')
  const searching = $derived(hist.query !== '')
  const lanes = $derived(searching ? 0 : Math.min(MAX_LANES, Math.max(1, ...hist.rows.map((r) => r.width))))

  onMount(() => {
    if (!commit) void hist.start()
    const t = setInterval(() => (now = Date.now()), 60_000)
    return () => {
      clearInterval(t)
      hist.stop()
    }
  })

  // Commits or branches changed (commit, switch, fetch): read the list again, keeping the place.
  // Staging and edits also bump git.changes but move no refs, so only a new HEAD or branch restarts, after a pause.
  let seenHead = untrack(() => `${git.status?.head}|${git.status?.branch}`)
  $effect(() => {
    const sig = `${git.status?.head}|${git.status?.branch}`
    if (sig === seenHead) return
    seenHead = sig
    if (commit) return
    const t = setTimeout(() => void hist.restart(), 400)
    return () => clearTimeout(t)
  })

  function nearEnd(_first: number, last: number) {
    if (last >= hist.commits.length - NEAR_END) void hist.more()
  }

  function submit(e: SubmitEvent) {
    e.preventDefault()
    selected = ''
    active = 0
    void hist.start(q, mode)
  }

  function clear() {
    q = ''
    selected = ''
    active = 0
    void hist.start()
  }

  const pick = (i: number) => {
    active = i
    selected = hist.commits[i]?.id ?? ''
  }

  async function move(to: number) {
    const i = Math.max(0, Math.min(hist.commits.length - 1, to))
    active = i
    list?.scrollToRow(i)
    await tick()
    // A far row is drawn a frame or two after the scroll; wait for it, else keep focus in the list.
    for (let n = 0; n < 20; n++) {
      const el = wrap?.querySelector<HTMLElement>(`[data-hrow="${i}"]`)
      if (el) return void el.focus()
      if (n === 0) wrap?.querySelector<HTMLElement>('[role="region"][tabindex]')?.focus()
      await new Promise((r) => requestAnimationFrame(r))
    }
    if (i >= hist.commits.length - NEAR_END) void hist.more()
  }

  function keys(e: KeyboardEvent) {
    if (e.altKey || e.ctrlKey || e.metaKey || !(e.target as HTMLElement).matches?.('[data-hrow]')) return
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : e.key === 'PageDown' ? 15 : e.key === 'PageUp' ? -15 : 0
    if (step) move(active + step)
    else if (e.key === 'Home') move(0)
    else if (e.key === 'End') move(hist.commits.length - 1)
    else return
    e.preventDefault()
  }

  const count = $derived(
    searching
      ? `${hist.commits.length}${hist.next ? ' or more' : ''} ${hist.commits.length === 1 ? 'match' : 'matches'}`
      : hist.total > 0
        ? `${hist.commits.length.toLocaleString()} of ${hist.total.toLocaleString()} commits read`
        : '',
  )
</script>

<div class="history" data-history-view>
  {#if commit}
    <CommitDetail {folder} id={commit} />
  {:else}
    <form class="bar" role="search" onsubmit={submit}>
      <div class="field">
        <label for="hq">Search the history</label>
        <input id="hq" class="input" type="search" maxlength="200" autocomplete="off" spellcheck="false" bind:value={q} />
      </div>
      <div class="field">
        <label for="hin">Look in</label>
        <span class="select"><select id="hin" class="input" bind:value={mode}>
          <option value="message">Message</option>
          <option value="author">Author</option>
          <option value="code">Code changed</option>
        </select></span>
      </div>
      <button type="submit" class="btn" disabled={q.trim() === ''}>Search</button>
      {#if searching || q}<button type="button" class="btn ghost" onclick={clear}>Clear</button>{/if}
    </form>
    <p class="status" role="status">
      {#if hist.loading && hist.commits.length === 0}{searching ? 'Searching…' : 'Reading the history…'}{:else}{count}{/if}
      {#if hist.stopped}. The search ran out of time, so these are the matches found so far.{/if}
    </p>
    {#if hist.error}
      <p class="error pad" role="alert">{hist.error} <button type="button" class="link" onclick={() => hist.start(hist.query, hist.mode)}>Try again</button></p>
    {/if}
    <div class="split" class:open={selected !== ''}>
      <div class="list" bind:this={wrap} onkeydown={keys} role="presentation">
        {#if hist.commits.length === 0 && !hist.loading && !hist.error}
          <p class="note">{searching ? `No commit matches “${hist.query}”. Try another word, or look in a different place.` : 'This repository has no commits yet. Commit some changes, and they show up here.'}</p>
        {:else}
          <VirtualList bind:this={list} count={hist.commits.length} label={searching ? 'Search results' : 'Commits'} estimate={32} follow={false} stick={false} jump="" onrange={nearEnd}>
            {#snippet row(i)}
              <HistoryRow commit={hist.commits[i]} graph={hist.rows[i]} {lanes} index={i} current={hist.commits[i].id === selected} tab={i === active} {now} onpick={pick} />
            {/snippet}
          </VirtualList>
        {/if}
        {#if hist.loading && hist.commits.length > 0}<p class="more" role="status">Reading more commits…</p>{/if}
      </div>
      {#if selected}
        <section class="detail" aria-label="Selected commit">
          <CommitDetail {folder} id={selected} />
        </section>
      {/if}
    </div>
  {/if}
</div>

<style>
  .history {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    container-type: inline-size;
    overflow-y: auto;
  }
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .bar .field:first-child {
    flex: 1 1 18ch;
  }
  .status,
  .note,
  .more,
  .pad {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .pad {
    color: var(--danger);
  }
  .note {
    padding: var(--s-4);
    font-size: var(--fs-m);
  }
  .more {
    position: absolute;
    right: var(--s-3);
    bottom: var(--s-2);
    background: var(--raised);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
  }
  .split {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .list {
    position: relative;
    display: flex;
    flex: 1 1 0;
    flex-direction: column;
    min-height: 160px;
  }
  .detail {
    flex: 0 1 45%;
    min-height: 0;
    overflow-y: auto;
    border-top: 1px solid var(--border);
    background: var(--surface);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
  @container (min-width: 900px) {
    .split {
      flex-direction: row;
    }
    .detail {
      flex: 0 0 min(420px, 40%);
      border-top: 0;
      border-left: 1px solid var(--border);
    }
  }
  @media (pointer: coarse) {
    .bar .btn {
      min-height: 44px;
    }
  }
</style>
