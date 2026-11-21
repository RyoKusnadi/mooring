<script lang="ts">
  // One file's changes: hunks with old and new line numbers, drawn only when in view. Working-copy and
  // staged diffs let the user pick lines to stage; a commit's diff is read-only.
  import { tick, untrack } from 'svelte'
  import DiffLine from './DiffLine.svelte'
  import DiffPickBar from './DiffPickBar.svelte'
  import VirtualList from './VirtualList.svelte'
  import { actions } from './lib/actions'
  import { diffCounts, diffRows } from './lib/diffrows'
  import { getDiff, gitCanRetry, gitError, type Diff } from './lib/gitapi'
  import { gitplus } from './lib/gitplus.svelte'
  import { git } from './lib/gitstore.svelte'
  import { getCommitDiff, shortId } from './lib/historyapi'
  import { LinePick } from './lib/linepick.svelte'
  import { blockRows, changeRows, stepRow, toPairs, whyNotChoose, type NoChoose } from './lib/linesel'
  import { baseName } from './lib/paths'
  import { app } from './lib/state.svelte'

  let { id, folder, path, staged, commit = '', loading = false, onflip }: { id: string; folder: string; path: string; staged: boolean; commit?: string; loading?: boolean; onflip?: (staged: boolean) => void } = $props()
  const flip = (to: boolean) => (onflip ? onflip(to) : actions.flipDiff(id, to))

  let diff = $state<Diff | null>(null)
  let error = $state('')
  let retry = $state(true)
  let follow = $state(false)
  let note = $state('')
  let wrap: HTMLElement | undefined = $state()
  let list: VirtualList | undefined = $state()
  let active = $state(-1)
  let swallow = false
  let ctl: AbortController | undefined
  const pick = new LinePick()
  const rows = $derived(diff ? diffRows(diff.hunks) : [])
  const counts = $derived(diff ? diffCounts(diff.hunks) : { added: 0, removed: 0 })
  const change = $derived(!commit && git.folder === folder ? git.status?.files.find((c) => c.path === path) : undefined)
  // Open in editor needs the file on disk. Without the status of this place (a worktree not shown),
  // a working-copy diff that only removes lines is a deleted file.
  const deleted = $derived(
    change ? change.unstaged === 'deleted' || (change.staged === 'deleted' && change.unstaged === '' && change.kind !== 'untracked')
      : !staged && !!diff && diff.hunks.length > 0 && diff.hunks.every((h) => h.newStart === 0 && h.newLines === 0),
  )
  const busy = $derived(git.busy !== '')
  const why = $derived<NoChoose>(diff && !commit && rows.length > 0 ? whyNotChoose(diff, staged, change?.kind === 'conflicted') : 'binary')
  const choosable = $derived(!commit && !!diff && rows.length > 0 && why === '')
  const tabRow = $derived(active >= 0 && rows[active] ? active : (changeRows(rows)[0] ?? -1))

  async function load() {
    ctl?.abort()
    const mine = (ctl = new AbortController())
    try {
      const d = commit ? await getCommitDiff(app.currentId, folder, commit, path, mine.signal) : await getDiff(app.currentId, folder, path, staged, undefined, mine.signal)
      if (mine.signal.aborted) return
      error = ''
      if (diff && d.version && d.version === diff.version) return
      if (pick.size > 0) note = 'The file changed, so the lines you chose were cleared. Choose them again.'
      pick.clear()
      diff = d
    } catch (e) {
      if (!mine.signal.aborted) {
        const g = gitError(e, 'Could not read this diff. Try again.')
        error = g.message
        retry = gitCanRetry(g.code)
      }
    }
  }

  // "Stage all" empties this side of the file: show the side that has the changes now, once.
  let flippedFrom: boolean | null = null
  $effect(() => {
    if (commit || !diff || diff.origPath) return
    if (rows.length > 0) return void (flippedFrom = null)
    if (!change || !(staged ? change.unstaged : change.staged) || flippedFrom === !staged) return
    flippedFrom = staged
    untrack(() => flip(!staged))
  })

  // Status is read every few seconds while a working-copy diff is open, and the diff with it.
  $effect(() => (commit ? undefined : git.hold()))

  // Read again whenever git state is read again for this project. A commit never changes.
  $effect(() => {
    if (!commit) void git.rev
    untrack(load)
    return () => ctl?.abort()
  })

  async function toggle() {
    // A staged rename is unstaged with both paths, or half of it stays staged.
    const orig = change?.origPath ?? diff?.origPath
    const paths = staged && orig ? [path, orig] : [path]
    const ok = await (staged ? git.unstage(paths, folder) : git.stage(paths, folder))
    if (ok) flip(!staged)
    else await load()
  }

  async function apply(chosen: Iterable<number>) {
    const pairs = toPairs(rows, chosen)
    if (pairs.length === 0 || !diff?.version) return
    note = ''
    const ok = await gitplus.stageLines(folder, path, staged, diff.version, pairs)
    const changed = !ok && git.failure?.code === 'DIFF_CHANGED'
    if (ok || changed) pick.clear()
    if (ok) note = `${pairs.length} ${pairs.length === 1 ? 'line' : 'lines'} ${staged ? 'unstaged' : 'staged'}.`
    else if (changed) note = 'The diff was read again. Choose the lines again.'
    await load()
    await tick()
    wrap?.querySelector<HTMLElement>('[role="region"]')?.focus()
  }

  const block = (i: number) => apply(blockRows(rows, i))

  function down(i: number, e: PointerEvent) {
    active = i
    if (e.pointerType !== 'mouse' || e.button !== 0 || (e.target as HTMLElement).closest('.text')) return
    swallow = true
    note = ''
    if (e.shiftKey) pick.extend(rows, i)
    else pick.dragStart(i)
    e.preventDefault()
    window.addEventListener('pointerup', () => (pick.dragEnd(), setTimeout(() => (swallow = false), 0)), { once: true })
    wrap?.querySelector<HTMLElement>(`[data-row="${i}"]`)?.focus()
  }

  function click(i: number) {
    if (swallow) return void (swallow = false)
    if (window.getSelection()?.toString()) return
    note = ''
    pick.toggle(i)
  }

  async function go(from: number, to: number, extend: boolean) {
    if (to < 0) return
    if (extend) {
      if (pick.anchor < 0) pick.anchor = from
      pick.extend(rows, to)
    }
    active = to
    list?.scrollToRow(to)
    await tick()
    wrap?.querySelector<HTMLElement>(`[data-row="${to}"]`)?.focus()
  }

  function keys(i: number, e: KeyboardEvent) {
    if (e.altKey || e.ctrlKey || e.metaKey) return
    note = ''
    if (e.key === ' ' || e.key === 'Enter') pick.toggle(i)
    else if (e.key === 'b' || e.key === 'B') pick.block(rows, i)
    else if (e.key === 'Escape') pick.clear()
    else if (e.key === 'ArrowDown') void go(i, stepRow(rows, i, 1), e.shiftKey)
    else if (e.key === 'ArrowUp') void go(i, stepRow(rows, i, -1), e.shiftKey)
    else return
    e.preventDefault()
  }
</script>

<div class="diff" bind:this={wrap} style:opacity={loading ? 0.5 : undefined} aria-busy={loading || undefined}>
  <header>
    <div class="what">
      <h2 title={path}>{baseName(path)}</h2>
      <p class="sub">
        {commit ? `As changed in ${shortId(commit)}` : staged ? 'Staged changes' : 'Changes not staged'}{folder && !commit ? ', in a worktree' : ''}{#if diff?.origPath}&nbsp;(was <code>{diff.origPath}</code>){/if}{#if diff && !diff.binary && !diff.tooLarge && rows.length > 0}: {counts.added} added, {counts.removed} removed{/if}
      </p>
      <p class="sub path" title={path}><code>{path}</code></p>
    </div>
    {#if !commit}
      <div class="acts">
        {#if choosable}<DiffPickBar count={pick.size} {staged} {busy} {note} onapply={() => apply(pick.chosen)} onclear={() => pick.clear()} />{/if}
        <button type="button" class="btn" disabled={busy || loading} onclick={toggle}>{staged ? 'Unstage file' : 'Stage file'}</button>
        <button type="button" class="btn" disabled={deleted} aria-describedby={deleted ? 'open-why' : undefined} onclick={() => actions.openPlaceFile(folder, path)}>Open in editor</button>
      </div>
      {#if deleted}<p class="sub why" id="open-why">This file is deleted, so there is nothing to open.</p>{/if}
    {/if}
  </header>
  {#if git.failure && !onflip}<p class="error pad" role="alert">{git.failure.message.split('\n')[0]}</p>{/if}
  {#if error}
    <p class="error pad" role="alert">{error}{#if retry} <button type="button" class="link" onclick={load}>Try again</button>{/if}</p>
  {:else if !diff}
    <p class="note" role="status">Loading the diff…</p>
  {:else if diff.binary}
    <p class="note">This is a binary file, so there are no lines to show. {commit ? '' : staged ? 'It is staged.' : 'Stage it to include it in a commit.'}</p>
  {:else if diff.tooLarge}
    <p class="note">This diff is over 2 MB, too large to show here. Open a terminal and run git diff for this file.</p>
  {:else if rows.length === 0 && diff.origPath}
    <p class="note">Renamed from <code>{diff.origPath}</code> with no change to the content.</p>
  {:else if rows.length === 0}
    <div class="note">
      <p>{staged ? 'Nothing of this file is staged now.' : 'This file has no unstaged changes now.'} It may have been committed, discarded or moved to the other side.</p>
      <button type="button" class="btn" onclick={() => flip(!staged)}>{staged ? 'Show unstaged changes' : 'Show staged changes'}</button>
    </div>
  {:else}
    {#if !commit && why === 'conflict'}<p class="sub pad">This file has merge conflicts, so lines cannot be chosen. Resolve it in the editor, then stage the file.</p>{/if}
    {#if !commit && why === 'rename'}<p class="sub pad">A staged rename is unstaged as a whole file. Use Unstage file to choose lines again.</p>{/if}
    <VirtualList bind:this={list} count={rows.length} label={`Changes in ${path}`} estimate={21} bind:follow jump="Jump to end">
      {#snippet row(i)}
        <DiffLine row={rows[i]} {i} {choosable} {staged} {busy} chosen={pick.has(i)} tab={i === tabRow} ondown={down} onenter={(k) => pick.dragging && pick.dragOver(rows, k)} onclick={click} onkey={keys} onblock={block} />
      {/snippet}
    </VirtualList>
  {/if}
</div>

<style>
  .diff {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .why {
    flex: 1 1 100%;
    margin: 0;
    color: var(--fg);
  }
  header {
    position: relative;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-1) var(--s-4);
    min-height: 40px;
    padding: var(--s-1) var(--s-4);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  /* Name, status and path share one row; the path is the part that gives way. */
  .what {
    display: flex;
    flex: 1 1 24ch;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--s-3);
    min-width: min(100%, 40ch);
  }
  .what .sub {
    flex: none;
  }
  .what .path {
    flex: 0 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow-wrap: normal;
  }
  .what h2 {
    flex: none;
    max-width: 100%;
  }
  @media (max-width: 900px) {
    .what {
      gap: 0 var(--s-3);
    }
    .what .sub {
      flex: 1 1 auto;
    }
  }
  h2 {
    margin: 0;
    overflow: hidden;
    font-size: var(--fs-m);
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .pad,
  .note {
    margin: 0;
    padding: var(--s-3) var(--s-4);
  }
  .note {
    display: grid;
    justify-items: start;
    gap: var(--s-3);
    max-width: 60ch;
    color: var(--muted);
  }
  .note p {
    margin: 0;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
</style>
