<script lang="ts">
  // One file of a chat's last turn, read-only: no staging. Lines can take comments for the agent.
  import DiffCommentLine from './DiffCommentLine.svelte'
  import VirtualList from './VirtualList.svelte'
  import { diffCounts, diffRows } from './lib/diffrows'
  import { gitError, type Diff } from './lib/gitapi'
  import { app } from './lib/state.svelte'
  import { getTurnDiff, type TurnFile } from './lib/turnapi'

  let { chat, file }: { chat: string; file: TurnFile } = $props()

  let diff = $state<Diff | null>(null)
  let error = $state('')
  let follow = $state(false)
  const rows = $derived(diff ? diffRows(diff.hunks) : [])
  const counts = $derived(diff ? diffCounts(diff.hunks) : { added: 0, removed: 0 })

  $effect(() => {
    let live = true
    getTurnDiff((n, a) => app.run(n, a), app.currentId, chat, file.path, file.origPath).then(
      (d) => live && (diff = d),
      (e: unknown) => live && (error = gitError(e, 'Could not read this diff. Try again.').message),
    )
    return () => (live = false)
  })
</script>

<div class="diff">
  <header>
    <h2 title={file.path}>{file.path}</h2>
    <p>{file.origPath ? `Renamed from ${file.origPath}. ` : ''}{#if diff && rows.length > 0}{counts.added} added, {counts.removed} removed{/if}</p>
  </header>
  {#if error}
    <p class="note error" role="alert">{error}</p>
  {:else if !diff}
    <p class="note" role="status">Loading the diff…</p>
  {:else if diff.binary}
    <p class="note">This is a binary file, so there is no text diff.</p>
  {:else if diff.tooLarge}
    <p class="note">This diff is over 2 MB, too large to show here. Open a terminal and run git diff.</p>
  {:else if rows.length === 0}
    <p class="note">No change to the content{file.origPath ? ' besides the new name' : ''}.</p>
  {:else}
    <VirtualList count={rows.length} label={`Changes in ${file.path}`} estimate={21} bind:follow jump="Jump to end">
      {#snippet row(i)}
        <DiffCommentLine row={rows[i]} path={file.path} {i} />
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
  header {
    padding: var(--s-2) var(--s-4);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  h2 {
    margin: 0;
    overflow: hidden;
    font-size: var(--fs-m);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  header p {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .note {
    margin: 0;
    padding: var(--s-4);
    color: var(--muted);
  }
  .note.error {
    color: var(--danger);
  }
</style>
