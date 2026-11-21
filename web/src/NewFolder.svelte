<script lang="ts">
  import ParentField from './ParentField.svelte'
  import { describe, type Field } from './lib/errors'
  import { app, type Project } from './lib/state.svelte'

  let { onDone }: { onDone: (p: Project) => void } = $props()

  let parent = $state(app.home)
  let name = $state('')
  let busy = $state(false)
  let error = $state<{ field: Field; message: string } | null>(null)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = null
    if (!name.trim()) {
      error = { field: 'name', message: 'Enter a name for the new folder.' }
      return
    }
    busy = true
    try {
      onDone(await app.addProject('project.create', { parent: parent.trim(), name: name.trim() }))
    } catch (err) {
      error = describe('create', err)
    } finally {
      busy = false
    }
  }
</script>

<form onsubmit={submit} novalidate>
  <ParentField id="new-parent" bind:value={parent} error={error?.field === 'parent' ? error.message : ''} />
  <div class="field">
    <label for="new-name">Folder name</label>
    <input id="new-name" class="input" bind:value={name} autocomplete="off" spellcheck="false" aria-invalid={error?.field === 'name' ? 'true' : undefined} aria-describedby={error?.field === 'name' ? 'new-name-error' : undefined} />
    {#if error?.field === 'name'}<p class="error" id="new-name-error" role="alert">{error.message}</p>{/if}
  </div>
  {#if error?.field === 'form'}<p class="error" role="alert">{error.message}</p>{/if}
  {#if busy && app.status !== 'live'}<p class="help" role="status">Waiting for the connection. This runs once Mooring is reachable again.</p>{/if}
  <div class="actions">
    <button type="submit" class="btn primary" disabled={busy}>{busy ? 'Creating…' : 'Create folder'}</button>
  </div>
</form>

<style>
  form {
    display: grid;
    gap: var(--s-3);
  }
  .actions {
    display: flex;
    gap: var(--s-2);
  }
</style>
