<script lang="ts">
  import ParentField from './ParentField.svelte'
  import { describe, type Field } from './lib/errors'
  import { app, type Project } from './lib/state.svelte'

  let { onDone }: { onDone: (p: Project) => void } = $props()

  let url = $state('')
  let parent = $state(app.home)
  let name = $state('')
  let busy = $state(false)
  let log: HTMLElement | undefined = $state()

  $effect(() => {
    void app.clone?.lines.length
    if (log) log.scrollTop = log.scrollHeight
  })
  let error = $state<{ field: Field; message: string } | null>(null)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = null
    if (!url.trim()) {
      error = { field: 'url', message: 'Enter the address of the repository to clone.' }
      return
    }
    busy = true
    try {
      const args: { url: string; parent: string; name?: string } = { url: url.trim(), parent: parent.trim() }
      if (name.trim()) args.name = name.trim()
      onDone(await app.cloneProject(args))
    } catch (err) {
      error = describe('clone', err, !!name.trim())
    } finally {
      busy = false
    }
  }
</script>

<form onsubmit={submit} novalidate>
  <div class="field">
    <label for="clone-url">Repository address</label>
    <input id="clone-url" class="input mono" bind:value={url} autocomplete="off" spellcheck="false" aria-invalid={error?.field === 'url' ? 'true' : undefined} aria-describedby={error?.field === 'url' ? 'clone-url-help clone-url-error' : 'clone-url-help'} />
    <p class="help" id="clone-url-help">Starts with https://, http://, ssh://, git:// or file://. Mooring cannot ask for a password.</p>
    {#if error?.field === 'url'}<p class="error" id="clone-url-error" role="alert">{error.message}</p>{/if}
  </div>
  <ParentField id="clone-parent" bind:value={parent} error={error?.field === 'parent' ? error.message : ''} />
  <div class="field">
    <label for="clone-name">Folder name (optional)</label>
    <input id="clone-name" class="input" bind:value={name} autocomplete="off" spellcheck="false" aria-invalid={error?.field === 'name' ? 'true' : undefined} aria-describedby="clone-name-help" />
    <p class="help" id="clone-name-help">Leave empty to use the name from the address.</p>
    {#if error?.field === 'name'}<p class="error" role="alert">{error.message}</p>{/if}
  </div>
  {#if app.clone && app.clone.lines.length > 0}
    <pre class="log" bind:this={log} role="log" aria-label="Clone progress">{app.clone.lines.join('\n')}</pre>
  {/if}
  {#if error?.field === 'form'}<p class="error" role="alert">{error.message}</p>{/if}
  {#if busy && app.status !== 'live'}<p class="help" role="status">Waiting for the connection. The clone continues once Mooring is reachable again.</p>{/if}
  <div class="actions">
    <button type="submit" class="btn primary" disabled={busy}>{busy ? 'Cloning…' : 'Clone repository'}</button>
  </div>
</form>

<style>
  form {
    display: grid;
    gap: var(--s-3);
  }
  .log {
    margin: 0;
    max-height: 160px;
    padding: var(--s-2);
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--bg);
    font: var(--fs-s) / 1.4 var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .actions {
    display: flex;
    gap: var(--s-2);
  }
</style>
