<script lang="ts">
  import Skeleton from './Skeleton.svelte'
  // What the sidebar shows of Changes: the branch, the staged and changed files, a short commit box,
  // and a button for the full view (line staging, stashes, history, push).
  import { untrack } from 'svelte'
  import SideChangeRow from './SideChangeRow.svelte'
  import SideCommit from './SideCommit.svelte'
  import { actions } from './lib/actions'
  import { changesPick } from './lib/changespick.svelte'
  import { pathsOf, stagedOf, unstagedOf } from './lib/gitapi'
  import { operationText } from './lib/gitoperation'
  import { git } from './lib/gitstore.svelte'
  import { app } from './lib/state.svelte'

  let { onNavigate }: { onNavigate: () => void } = $props()

  const status = $derived(git.status)
  const count = $derived(status?.repo ? status.files.length : 0)
  const staged = $derived(status?.repo ? stagedOf(status) : [])
  const unstaged = $derived(status?.repo ? unstagedOf(status) : [])
  const busy = $derived(git.busy !== '')

  // The project on screen is the project whose changes are read.
  $effect(() => {
    const id = app.ready ? app.currentId : ''
    untrack(() => git.select(id))
  })

  // Read every few seconds while this summary is on screen.
  $effect(() => {
    if (git.view === 'changes') return git.hold()
  })

  const show = (path = '', isStaged = false) => {
    if (path) changesPick.ask(path, isStaged)
    actions.showChanges()
    onNavigate()
  }
</script>

<svelte:window onfocus={() => git.refresh()} />

{#if app.ready && app.current}
  <section aria-label="Changes summary">
    <div class="top">
      {#if status?.repo}
        <p class="line">
          <code>{status.detached ? `detached at ${status.head}` : status.branch || 'no commits yet'}</code>
          <span class="count">{count === 0 ? 'No changes' : count === 1 ? '1 changed file' : `${count} changed files`}</span>
        </p>
      {/if}
      <button type="button" class="btn ghost small open" title="Line staging, stashes, history and push" onclick={() => show()}>Open changes</button>
    </div>
    <div class="scroll">
      {#if git.readError}
        <p class="hint" role="alert">{git.readError}</p>
      {:else if !status}
        <div class="hint" role="status"><Skeleton rows={3} label="Reading changes" /></div>
      {:else if !status.repo}
        <p class="hint">This folder is not in a git repository. Run git init in a terminal to start one.</p>
      {:else}
        {#if status.operation}<p class="hint warn" role="status">{operationText(status.operation, status.files.filter((c) => c.kind === 'conflicted').length).title} Finish it in Changes.</p>{:else if status.conflicted}<p class="hint warn" role="status">Some files have merge conflicts.</p>{/if}
        {#if staged.length > 0}
          <div class="bar">
            <h3>Staged <span class="n">{staged.length}</span></h3>
            <button type="button" class="btn ghost small" disabled={busy} onclick={() => git.unstage([''])}>Unstage all</button>
          </div>
          <ul aria-label="Staged files">
            {#each staged as c (c.path)}
              <SideChangeRow {c} staged {busy} onopen={() => show(c.path, true)} ontoggle={() => git.unstage(pathsOf(c, true))} />
            {/each}
          </ul>
        {/if}
        {#if unstaged.length > 0}
          <div class="bar">
            <h3>Changes <span class="n">{unstaged.length}</span></h3>
            <button type="button" class="btn ghost small" disabled={busy || unstaged.every((c) => c.kind === 'conflicted')} onclick={() => git.stage([''])}>Stage all</button>
          </div>
          <ul aria-label="Changed files">
            {#each unstaged as c (c.path)}
              <SideChangeRow {c} staged={false} {busy} onopen={() => show(c.path, false)} ontoggle={() => git.stage([c.path])} />
            {/each}
          </ul>
        {/if}
        {#if status.truncated}<p class="hint">The list is cut short because there are too many changes. Commit or discard some in Changes, then refresh.</p>{/if}
      {/if}
    </div>
    {#if status?.repo}<div class="foot"><SideCommit /></div>{/if}
  </section>
{/if}

<style>
  section {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-1);
    padding: var(--s-2) var(--s-2) 0 var(--s-3);
  }
  .open {
    margin-left: auto;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    padding: var(--s-1) var(--s-2) var(--s-4);
    overflow-y: auto;
  }
  /* The list fades into the sidebar at its bottom edge instead of ending in a hard cut. */
  .scroll::after {
    content: '';
    display: block;
    position: sticky;
    bottom: 0;
    height: var(--s-4);
    margin-top: calc(-1 * var(--s-4));
    background: linear-gradient(to bottom, transparent, var(--surface));
    pointer-events: none;
  }
  .line,
  .hint {
    margin: 0;
    padding: 0 var(--s-1);
    font-size: var(--fs-s);
  }
  .line {
    display: grid;
    gap: 2px;
    min-width: 0;
    padding: 0;
  }
  code {
    font-weight: var(--fw-semibold);
    overflow-wrap: anywhere;
  }
  .count,
  .hint {
    color: var(--muted);
  }
  .hint {
    margin-top: var(--s-1);
  }
  .warn {
    color: var(--attention);
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-1);
    margin-top: var(--s-2);
    padding-left: var(--s-1);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .n {
    color: var(--muted);
    font-weight: var(--fw-regular);
  }
  ul {
    margin: 0;
    padding: 0;
  }
  .small {
    min-height: var(--control-h-s);
    padding: 0 var(--s-1);
    font-size: var(--fs-s);
  }
  .foot {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-2);
    border-top: 1px solid var(--border);
  }
  @media (pointer: coarse) {
    .small {
      min-height: 44px;
    }
  }
</style>
