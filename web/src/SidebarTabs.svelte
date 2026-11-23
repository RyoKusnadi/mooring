<script lang="ts">
  // Files, Changes and Board: three text tabs. The chosen one is dark with a line under it; no fill, so the
  // only filled row in the sidebar is the chosen project.
  import { switchLabel } from './lib/boardfmt'
  import { git } from './lib/gitstore.svelte'

  let { counts, badge }: { counts: { waiting: number; finished: number }; badge: number } = $props()
  const changed = $derived(git.status?.repo ? git.status.files.length : 0)
</script>

<div class="tabs" role="group" aria-label="Show">
  <button type="button" aria-pressed={git.view === 'files'} onclick={() => (git.view = 'files')}>Files</button>
  <button type="button" aria-pressed={git.view === 'changes'} aria-label={changed > 0 ? `Changes, ${changed} ${changed === 1 ? 'file' : 'files'}` : undefined} onclick={() => (git.view = 'changes')}>
    Changes{#if changed > 0}<span class="n">{changed}</span>{/if}
  </button>
  <button type="button" aria-pressed={git.view === 'board'} aria-label={switchLabel(counts)} onclick={() => (git.view = 'board')}>
    Board{#if badge > 0}<span class="n">{badge}</span>{/if}
  </button>
</div>

<style>
  .tabs {
    display: flex;
    gap: var(--s-1);
    margin-top: var(--s-2);
    padding: 0 var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  button {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s) var(--r-s) 0 0;
    background: none;
    color: var(--muted);
    font: inherit;
    font-weight: var(--fw-medium);
    transition: color var(--ease);
  }
  button:hover {
    color: var(--fg);
  }
  button[aria-pressed='true'] {
    color: var(--fg);
  }
  button::after {
    content: '';
    position: absolute;
    right: var(--s-2);
    bottom: -1px;
    left: var(--s-2);
    height: 2px;
    background: transparent;
    transition: background-color var(--ease);
  }
  button[aria-pressed='true']::after {
    background: var(--accent);
  }
  .n {
    color: var(--muted);
    font-weight: var(--fw-regular);
  }
</style>
