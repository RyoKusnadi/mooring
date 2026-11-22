<script lang="ts">
  import Icon from './Icon.svelte'
  import PaneMenu, { type Entry } from './PaneMenu.svelte'
  import { actions } from './lib/actions'
  import { files } from './lib/filestore.svelte'
  import { showKey } from './lib/keys'
  import { app } from './lib/state.svelte'

  let { number, total }: { number: number; total: number } = $props()
  const gone = $derived(files.rootMissing(app.currentId))

  const create = $derived<Entry[]>([
    { label: 'Terminal', icon: 'terminal', hint: showKey('Alt+N'), disabled: gone, run: actions.newTerminal },
    { label: 'Chat', icon: 'chat', hint: showKey('Alt+A'), disabled: gone, run: actions.newChat },
    { label: 'Open file', icon: 'file', hint: showKey('Alt+O'), run: () => (files.finder = 'file') },
  ])
  const splits = $derived<Entry[]>([
    { label: 'Split right', icon: 'splitRight', hint: showKey('Alt+\\'), disabled: gone, run: () => actions.split('row') },
    { label: 'Split down', icon: 'splitDown', hint: showKey('Alt+-'), disabled: gone, run: () => actions.split('column') },
  ])
</script>

<div class="actions">
  <PaneMenu label="New tab" icon="plus" items={create} mark="new" />
  {#if !app.narrow}
    <span class="wide">
      <button type="button" class="btn ghost icon" aria-label="Split right" title={`Split right (${showKey('Alt+\\')})`} disabled={gone} onclick={() => actions.split('row')}><Icon name="splitRight" /></button>
      <button type="button" class="btn ghost icon" aria-label="Split down" title={`Split down (${showKey('Alt+-')})`} disabled={gone} onclick={() => actions.split('column')}><Icon name="splitDown" /></button>
    </span>
    <span class="narrow-only"><PaneMenu label="More pane actions" icon="more" items={splits} /></span>
  {:else if total > 1}
    <button type="button" class="btn ghost" aria-label="Show next pane" onclick={actions.nextPane}>Pane {number} of {total}</button>
  {/if}
</div>

<style>
  .actions {
    display: flex;
    flex: none;
    align-items: center;
    gap: var(--s-1);
    padding: 0 var(--s-2);
  }
  .wide {
    display: contents;
  }
  .narrow-only {
    display: none;
  }
  .icon {
    width: var(--control-h);
    padding: 0;
    color: var(--muted);
  }
  .icon:hover:not(:disabled) {
    color: var(--fg);
  }
  /* A narrow split: the two split buttons fold into a "…" menu. */
  @container pane (max-width: 400px) {
    .wide {
      display: none;
    }
    .narrow-only {
      display: contents;
    }
  }
</style>
