<script lang="ts">
  // Per project: whether Mooring records the folder before and after each agent turn.
  import type { Settings } from './lib/settingsapi'
  import { settings } from './lib/settingsstore.svelte'
  import { app } from './lib/state.svelte'

  let { save }: { save: (change: Partial<Settings>) => Promise<void> } = $props()

  const project = $derived(app.current)
  const on = $derived(!!project && !settings.value.checkpointsOff[project.id])

  // The server replaces the whole map, so the full map goes back each time.
  function set(want: boolean) {
    if (!project) return
    const next = { ...settings.value.checkpointsOff }
    if (want) delete next[project.id]
    else next[project.id] = true
    void save({ checkpointsOff: next })
  }
</script>

{#if project}
  <label class="row">
    <input type="checkbox" checked={on} onchange={(e) => set(e.currentTarget.checked)} />
    <span>
      <strong>Save a checkpoint before each agent turn in {project.name}</strong>
      <span class="help">Mooring records the folder before and after each turn, without touching your staged changes, so you can see what the agent changed. Open Changes and choose Last turn.</span>
    </span>
  </label>
{/if}

<style>
  .row {
    display: flex;
    gap: var(--s-3);
    align-items: flex-start;
    margin-bottom: var(--s-4);
  }
  .row input {
    margin-top: var(--s-1);
    flex: none;
  }
  .row span {
    display: grid;
    gap: var(--s-1);
  }
</style>
