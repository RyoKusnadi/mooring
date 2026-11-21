<script lang="ts">
  import Node from './Node.svelte'
  import Pane from './Pane.svelte'
  import Unreachable from './Unreachable.svelte'
  import { focusedGroup } from './lib/layout'
  import { app } from './lib/state.svelte'

  let { onAdd }: { onAdd: () => void } = $props()

  const group = $derived(app.layout ? focusedGroup(app.layout) : null)
</script>

<div class="workspace">
  {#if !app.ready}
    <div class="empty"><Unreachable /></div>
  {:else if app.projects.length > 0 && !app.current}
    <p class="empty">Pick a project from the list to open it.</p>
  {:else if !app.current}
    <div class="empty">
      <h1>No projects yet</h1>
      <p>A project is a folder on the computer running Mooring. Add one to open terminals, agent chats and files in it.</p>
      <button type="button" class="btn primary" onclick={onAdd}>Add project</button>
    </div>
  {:else if app.layout}
    {#key app.currentId}
      {#if app.narrow && group}
        <Pane {group} />
      {:else}
        <Node node={app.layout.root} />
      {/if}
    {/key}
  {/if}
</div>

<style>
  .workspace {
    min-width: 0;
    min-height: 0;
    height: 100%;
  }
  .empty {
    display: grid;
    justify-items: center;
    gap: var(--s-3);
    max-width: 560px;
    margin: 0 auto;
    padding: var(--s-6) var(--s-4);
    text-align: center;
    color: var(--muted);
  }
  h1 {
    margin: 0;
    color: var(--fg);
    font-size: var(--fs-l);
    line-height: 1.25;
  }
  p {
    margin: 0;
  }
</style>
