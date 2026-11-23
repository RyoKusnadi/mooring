<script lang="ts">
  import { onMount } from 'svelte'
  import CloneRepo from './CloneRepo.svelte'
  import FolderBrowser from './FolderBrowser.svelte'
  import NewFolder from './NewFolder.svelte'
  import { describe } from './lib/errors'
  import { app, type Project } from './lib/state.svelte'

  let { onClose }: { onClose: () => void } = $props()

  type Mode = 'open' | 'create' | 'clone'
  const modes: { id: Mode; label: string }[] = [
    { id: 'open', label: 'Open a folder' },
    { id: 'create', label: 'New folder' },
    { id: 'clone', label: 'Clone a repository' },
  ]

  let dialog: HTMLDialogElement | undefined = $state()
  let mode = $state<Mode>('open')
  let busy = $state(false)
  let openError = $state('')

  onMount(() => dialog?.showModal())

  function done(p: Project) {
    app.select(p.id)
    onClose()
  }

  async function openFolder(path: string) {
    busy = true
    openError = ''
    try {
      done(await app.addProject('project.add', { path }))
    } catch (err) {
      openError = describe('open', err).message
    } finally {
      busy = false
    }
  }

  function tabKeys(e: KeyboardEvent) {
    const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const i = modes.findIndex((m) => m.id === mode)
    const next = modes[(i + step + modes.length) % modes.length].id
    mode = next
    document.getElementById('add-tab-' + next)?.focus()
  }
</script>

<dialog bind:this={dialog} class="dlg wide" aria-labelledby="add-title" onclose={onClose} oncancel={() => (busy = false)}>
  <header>
    <h2 id="add-title">Add project</h2>
    <button type="button" class="btn ghost" aria-label="Close" onclick={() => dialog?.close()}>×</button>
  </header>
  <div class="modes" role="tablist" aria-label="How to add a project" tabindex="-1" onkeydown={tabKeys}>
    {#each modes as m (m.id)}
      <button type="button" role="tab" id={'add-tab-' + m.id} aria-selected={mode === m.id} aria-controls="add-panel" tabindex={mode === m.id ? 0 : -1} onclick={() => (mode = m.id)}>{m.label}</button>
    {/each}
  </div>
  <div class="panel" id="add-panel" role="tabpanel" aria-labelledby={'add-tab-' + mode}>
    {#if mode === 'open'}
      <FolderBrowser start={app.home} actionLabel="Open this folder" {busy} error={openError} onPick={openFolder} />
    {:else if mode === 'create'}
      <NewFolder onDone={done} />
    {:else}
      <CloneRepo onDone={done} />
    {/if}
  </div>
  <footer><button type="button" class="btn" onclick={() => dialog?.close()}>Close</button></footer>
</dialog>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--s-3) var(--s-3) 0 var(--s-4);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-l);
    line-height: var(--lh-l);
  }
  .modes {
    display: flex;
    gap: var(--s-1);
    padding: var(--s-3) var(--s-4) 0;
    border-bottom: 1px solid var(--border);
  }
  .modes button {
    flex: 1 1 0;
    min-height: var(--control-h);
    padding: var(--s-1) var(--s-2);
    text-align: center;
    border: 0;
    border-radius: var(--r-s) var(--r-s) 0 0;
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-weight: var(--fw-semibold);
    transition: background-color var(--ease);
  }
  .modes button:hover {
    background: var(--hover);
  }
  .modes button[aria-selected='true'] {
    background: var(--selected);
    color: var(--fg);
  }
  .panel {
    padding: var(--s-4);
  }
</style>
