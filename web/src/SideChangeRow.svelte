<script lang="ts">
  // One changed file in the sidebar list: letter, name, dimmed folder, and a stage or unstage button.
  import Icon from './Icon.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import { sideLetter, sideWord, type Change } from './lib/gitapi'
  import { baseName, parentPath } from './lib/paths'

  let { c, staged, busy, onopen, ontoggle }: { c: Change; staged: boolean; busy: boolean; onopen: () => void; ontoggle: () => void } = $props()

  const dir = $derived(c.origPath && staged ? `was ${c.origPath}` : parentPath(c.path))
</script>

<li>
  <button type="button" class="open" title={c.path} aria-label={`Show changes to ${c.path}, ${sideWord(c, staged)}`} onclick={onopen}>
    <StatusBadge letter={sideLetter(c, staged)} />
    <span class="name">{baseName(c.path)}</span>
    {#if dir}<span class="dir">{dir}</span>{/if}
  </button>
  {#if c.kind !== 'conflicted'}
    <button type="button" class="icon-btn" disabled={busy} aria-label={`${staged ? 'Unstage' : 'Stage'} ${c.path}`} title={staged ? 'Unstage' : 'Stage'} onclick={ontoggle}><Icon name={staged ? 'minus' : 'plus'} /></button>
  {/if}
</li>

<style>
  li {
    display: flex;
    align-items: center;
    border-radius: var(--r-s);
    list-style: none;
  }
  li:hover {
    background: var(--hover);
  }
  .open {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    min-height: var(--row-h);
    padding: 0 var(--s-1);
    border: 0;
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  /* The folder gives way first: it takes what the name leaves, and goes when there is none. */
  .name,
  .dir {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dir {
    flex: 1 1 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
