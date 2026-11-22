<script lang="ts">
  // What the sidebar keeps of Changes: the branch, how many files changed, and a button for the full view.
  import { untrack } from 'svelte'
  import { actions } from './lib/actions'
  import { operationText } from './lib/gitoperation'
  import { git } from './lib/gitstore.svelte'
  import { app } from './lib/state.svelte'

  let { onNavigate }: { onNavigate: () => void } = $props()

  const status = $derived(git.status)
  const count = $derived(status?.repo ? status.files.length : 0)

  // The project on screen is the project whose changes are read.
  $effect(() => {
    const id = app.ready ? app.currentId : ''
    untrack(() => git.select(id))
  })

  // Read every few seconds while this summary is on screen.
  $effect(() => {
    if (git.view === 'changes') return git.hold()
  })
</script>

<svelte:window onfocus={() => git.refresh()} />

{#if app.ready && app.current}
  <section aria-label="Changes summary">
    {#if git.readError}
      <p class="hint" role="alert">{git.readError}</p>
    {:else if !status}
      <p class="hint" role="status">Reading changes…</p>
    {:else if !status.repo}
      <p class="hint">This folder is not in a git repository. Run git init in a terminal to start one.</p>
    {:else}
      <p class="line">
        <code>{status.detached ? `detached at ${status.head}` : status.branch || 'no commits yet'}</code>
        <span class="count">{count === 0 ? 'No changes' : count === 1 ? '1 changed file' : `${count} changed files`}</span>
      </p>
      {#if status.operation}<p class="hint warn" role="status">{operationText(status.operation, status.files.filter((c) => c.kind === 'conflicted').length).title} Finish it in Changes.</p>{:else if status.conflicted}<p class="hint warn" role="status">Some files have merge conflicts.</p>{/if}
    {/if}
    <div class="act">
      <button type="button" class="btn" onclick={() => (actions.showChanges(), onNavigate())}>Open changes</button>
    </div>
  </section>
{/if}

<style>
  section {
    display: grid;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
  }
  .line {
    display: grid;
    gap: 2px;
    margin: 0;
    font-size: var(--fs-s);
  }
  code {
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .count,
  .hint {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .warn {
    color: var(--attention);
  }
</style>
