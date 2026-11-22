<script lang="ts">
  import { onMount } from 'svelte'
  import { lsp } from './lib/lspstore.svelte'
  import { formatMb, parseEndings, placeLabel, slugId, splitCommand, type LspServer } from './lib/lspmatch'
  import { app } from './lib/state.svelte'

  let { onClose }: { onClose: () => void } = $props()

  let dialog: HTMLDialogElement | undefined = $state()
  let busy = $state('')
  let error = $state('')
  let errorFor = $state('')
  let name = $state('')
  let endings = $state('')
  let command = $state('')
  let formError = $state('')

  onMount(() => {
    dialog?.showModal()
    void lsp.load(true).catch(() => ((errorFor = ''), (error = 'Could not read the language servers. Check that Mooring is running.')))
  })

  const STATE: Record<LspServer['state'], string> = {
    ready: 'Ready',
    needs_consent: 'Waiting for your permission',
    not_installed: 'Not installed',
    installing: 'Installing',
  }

  async function act(id: string, run: () => Promise<unknown>) {
    busy = id
    error = ''
    errorFor = id
    try {
      await run()
    } catch (e) {
      error = e instanceof Error ? e.message : 'That did not work. Try again.'
    } finally {
      busy = ''
    }
  }

  async function addCustom(e: SubmitEvent) {
    e.preventDefault()
    const id = slugId(name)
    const list = parseEndings(endings)
    const argv = splitCommand(command)
    formError = !id ? 'Enter a name with letters or numbers.' : !list.length ? 'Enter at least one file ending, such as .rs.' : !argv.length ? 'Enter the command that starts the server.' : ''
    if (formError) return
    await act('form', () => lsp.setCustom({ id, name: name.trim(), endings: list, languageId: id, command: argv }))
    if (!error) [name, endings, command] = ['', '', '']
    else [formError, error] = [error, '']
  }

  const projectName = (id: string) => app.projects.find((p) => p.id === id)?.name ?? id
  const serverName = (id: string) => lsp.server(id)?.name ?? id
</script>

<dialog bind:this={dialog} class="dlg wide" aria-labelledby="lsp-title" onclose={onClose}>
  <header>
    <h2 id="lsp-title">Language servers</h2>
    <button type="button" class="btn ghost" aria-label="Close" onclick={() => dialog?.close()}>×</button>
  </header>
  <div class="body">
    {#if error && !errorFor}<p class="error" role="alert">{error}</p>{/if}

    <h3>Servers</h3>
    {#if !lsp.loaded}
      <p class="muted" role="status">Loading…</p>
    {:else}
      <ul class="list">
        {#each lsp.servers as s (s.id)}
          <li>
            <div class="what">
              <strong>{s.name}</strong>
              <span class="muted">{s.endings.join(' ')}{s.custom ? ', added by you' : ''}</span>
              <span class="state">{STATE[s.state]}{#if s.state === 'installing' && lsp.progress[s.id]}: {lsp.progress[s.id]}{/if}</span>
              {#if s.state === 'needs_consent'}<span class="muted">Runs <code>{s.command.join(' ')}</code></span>
              {:else if s.path}<code>{s.path}</code>
              {:else if s.state === 'not_installed' && !s.installable}<span class="muted">{s.installHint}</span>{/if}
              {#if error && errorFor === s.id}<span class="error" role="alert">{error}</span>{/if}
            </div>
            <div class="acts">
              {#if s.state === 'not_installed' && s.installable}
                <button type="button" class="btn" disabled={busy === s.id} aria-label="Install {s.name}" onclick={() => act(s.id, () => lsp.install(s.id))}>Install</button>
              {/if}
              {#if s.state === 'needs_consent'}
                <button type="button" class="btn primary" disabled={busy === s.id} aria-label="Allow {s.name}" onclick={() => act(s.id, () => lsp.allow(s.id, s.command))}>Allow</button>
              {/if}
              {#if s.custom}
                <button type="button" class="btn" disabled={busy === s.id} aria-label="Remove {s.name}" onclick={() => act(s.id, () => lsp.removeCustom(s.id))}>Remove</button>
              {/if}
            </div>
          </li>
        {/each}
      </ul>

      <h3>Running now</h3>
      {#if lsp.running.length === 0}
        <p class="muted">No language server is running.</p>
      {:else}
        <ul class="list" data-testid="lsp-running">
          {#each lsp.running as r (r.projectId + r.folder + r.id)}
            <li>
              <div class="what">
                <strong>{serverName(r.id)}</strong>
                <span class="muted">{projectName(r.projectId)}, {placeLabel(r.folder)}</span>
                <span class="state">{r.mb ? formatMb(r.mb) : 'Starting'}, {r.clients} {r.clients === 1 ? 'tab' : 'tabs'}</span>
                {#if error && errorFor === r.id + r.folder}<span class="error" role="alert">{error}</span>{/if}
              </div>
              <div class="acts">
                <button type="button" class="btn" disabled={busy === r.id + r.folder} aria-label="Stop {serverName(r.id)} in {projectName(r.projectId)}" onclick={() => act(r.id + r.folder, () => lsp.stop(r.projectId, r.folder, r.id))}>Stop</button>
              </div>
            </li>
          {/each}
        </ul>
      {/if}

      <h3>Add a server</h3>
      <form onsubmit={addCustom}>
        <div class="field">
          <label for="lsp-name">Name</label>
          <input id="lsp-name" class="input" bind:value={name} autocomplete="off" spellcheck="false" />
        </div>
        <div class="field">
          <label for="lsp-endings">File endings</label>
          <input id="lsp-endings" class="input" bind:value={endings} placeholder=".rs, .rlib" autocomplete="off" spellcheck="false" />
        </div>
        <div class="field">
          <label for="lsp-command">Command</label>
          <input id="lsp-command" class="input mono" bind:value={command} placeholder="rust-analyzer" autocomplete="off" spellcheck="false" />
          <p class="muted hint">Mooring asks before it runs it the first time.</p>
        </div>
        {#if formError}<p class="error" role="alert">{formError}</p>{/if}
        <button type="submit" class="btn primary" disabled={busy === 'form'}>Add server</button>
      </form>
    {/if}
  </div>
  <footer><button type="button" class="btn primary" onclick={() => dialog?.close()}>Done</button></footer>
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
    line-height: 1.25;
  }
  h3 {
    margin: var(--s-4) 0 var(--s-2);
    font-size: var(--fs-m);
  }
  .body {
    padding: var(--s-2) var(--s-4) var(--s-4);
  }
  .list {
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) 0;
    border-bottom: 1px solid var(--border);
  }
  .what {
    display: flex;
    flex: 1 1 24ch;
    flex-direction: column;
    min-width: 0;
    gap: var(--s-1);
    font-size: var(--fs-s);
  }
  .what strong {
    font-size: var(--fs-m);
  }
  code {
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  .acts {
    display: flex;
    gap: var(--s-2);
  }
  .muted {
    color: var(--muted);
  }
  .hint {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-s);
  }
  form {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-3);
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    align-self: stretch;
  }
  label {
    font-size: var(--fs-s);
    font-weight: 600;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .error {
    margin: var(--s-1) 0 0;
    color: var(--danger);
  }
</style>
