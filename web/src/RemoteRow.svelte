<script lang="ts">
  // One entry of a remote folder: open it, download it, rename it, or delete it after a question.
  import { tick } from 'svelte'
  import { actions } from './lib/actions'
  import { remoteTabsIn, retargetRemote } from './lib/remotedir'
  import { app } from './lib/state.svelte'
  import { CommandError } from './lib/events'
  import type { Entry } from './lib/filesapi'
  import { formatBytes, joinPath, nameProblem } from './lib/paths'
  import { downloadUrl } from './lib/sshapi'
  import { ssh } from './lib/sshstore.svelte'

  let { entry, dir, conn, host, onOpenDir, onChanged }: { entry: Entry; dir: string; conn: string; host: string; onOpenDir: (path: string) => void; onChanged: (message?: string) => void } = $props()

  const full = $derived(joinPath(dir, entry.name))
  const isDir = $derived(entry.kind === 'dir')
  const openable = $derived(!entry.unsafe && (entry.kind === 'dir' || entry.kind === 'file' || entry.kind === 'link'))
  let mode = $state<'view' | 'rename' | 'delete'>('view')
  let name = $state('')
  let error = $state('')
  let busy = $state(false)
  let notEmpty = $state(false)
  let box: HTMLElement | undefined = $state()

  async function enter(m: 'rename' | 'delete') {
    error = ''
    notEmpty = false
    name = entry.name
    mode = m
    await tick()
    const el = box?.querySelector<HTMLInputElement | HTMLButtonElement>(m === 'rename' ? 'input' : '[data-keep]')
    el?.focus()
    if (el instanceof HTMLInputElement) el.select()
  }

  async function leave() {
    mode = 'view'
    await tick()
    box?.querySelector<HTMLElement>('[data-act="name"]')?.focus()
  }

  async function run(fn: () => Promise<unknown>, done: string) {
    busy = true
    error = ''
    try {
      await fn()
      mode = 'view'
      onChanged(done)
    } catch (e) {
      error = e instanceof Error ? e.message : 'That did not work. Try again.'
      if (mode === 'delete' && e instanceof CommandError && /not empty/i.test(e.message)) notEmpty = true
    } finally {
      busy = false
    }
  }

  function rename(e: SubmitEvent) {
    e.preventDefault()
    const n = name.trim()
    error = n === entry.name ? 'Type a different name.' : nameProblem(n)
    if (!error) {
      const to = joinPath(dir, n)
      void run(() => ssh.cmd('ssh.fileRename', { conn, from: full, to }).then(() => followRename(to)), `Renamed ${entry.name} to ${n}.`)
    }
  }

  // Open editors of the file follow a rename and close after a delete.
  const followRename = (to: string) => app.edit((l) => retargetRemote(l, host, full, to))
  function followDelete() {
    const l = app.layout
    if (l) for (const t of remoteTabsIn(l, host, full)) actions.closeTab(t.id)
  }

  const esc = (e: KeyboardEvent) => e.key === 'Escape' && (e.stopPropagation(), leave())
  const remove = () => run(() => ssh.cmd('ssh.fileDelete', { conn, path: full, recursive: notEmpty }).then(followDelete), `Deleted ${entry.name}.`)
  const open = () => (isDir ? onOpenDir(full) : actions.openRemoteFile(host, full))
</script>

<li bind:this={box} class:dim={entry.hidden}>
  {#if mode === 'rename'}
    <form onsubmit={rename} aria-label="Rename {entry.name}">
      <label class="sr" for="rn-{entry.name}">New name for {entry.name}</label>
      <input id="rn-{entry.name}" class="input mono" bind:value={name} autocomplete="off" spellcheck="false" onkeydown={(e) => e.key === 'Escape' && (e.stopPropagation(), leave())} />
      <button type="submit" class="btn primary" disabled={busy}>Rename</button>
      <button type="button" class="btn" onclick={leave}>Cancel</button>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    </form>
  {:else if mode === 'delete'}
    <div class="confirm" role="group" aria-label="Delete {entry.name}">
      <p>
        {#if notEmpty}{entry.name} has files in it. Delete the folder and everything inside?{:else}Delete {entry.name}? The host has no trash, so this cannot be undone.{/if}
      </p>
      <button type="button" class="btn danger solid" disabled={busy} onclick={remove} onkeydown={esc}>{notEmpty ? 'Delete everything' : 'Delete'}</button>
      <button type="button" class="btn" data-keep onclick={leave} onkeydown={esc}>Keep</button>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
    </div>
  {:else}
    <button type="button" class="name" data-act="name" disabled={!openable} onclick={open} aria-label={isDir ? `Open folder ${entry.name}` : `Open ${entry.name}`}>
      <span class="kind" aria-hidden="true">{isDir ? '▸' : ''}</span>
      <span class="text">{entry.name}{isDir ? '/' : ''}</span>
      {#if entry.unsafe}<span class="meta">Leads outside the root</span>{:else if !isDir && entry.kind === 'file'}<span class="meta">{formatBytes(entry.size)}</span>{/if}
    </button>
    <span class="acts">
      {#if !isDir && openable}<a class="btn" href={downloadUrl(conn, full)} download={entry.name} aria-label="Download {entry.name}">Download</a>{/if}
      <button type="button" class="btn" aria-label="Rename {entry.name}" onclick={() => enter('rename')}>Rename</button>
      <button type="button" class="btn danger" aria-label="Delete {entry.name}" onclick={() => enter('delete')}>Delete</button>
    </span>
  {/if}
</li>

<style>
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-2);
    padding: 0 var(--s-2);
    border-bottom: 1px solid var(--border);
  }
  li:hover {
    background: var(--hover);
  }
  li.dim .text {
    color: var(--muted);
  }
  .name {
    display: flex;
    flex: 1 1 14em;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    min-height: var(--control-h);
    padding: 0;
    border: 0;
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .name:disabled {
    color: var(--muted);
    cursor: not-allowed;
  }
  .kind {
    flex: none;
    width: 1em;
    color: var(--muted);
  }
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    flex: none;
    margin-left: auto;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
  }
  .acts .btn {
    border-color: transparent;
    font-size: var(--fs-s);
    text-decoration: none;
  }
  form,
  .confirm {
    display: flex;
    flex: 1;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) 0;
  }
  form .input {
    flex: 1 1 12em;
    width: auto;
  }
  .confirm p {
    flex: 1 1 100%;
    margin: 0;
  }
  form .error {
    flex: 1 1 100%;
  }
  @media (max-width: 900px) {
    .acts {
      display: grid;
      grid-auto-flow: column;
      grid-auto-columns: 1fr;
      flex: 1 1 100%;
      padding-bottom: var(--s-2);
    }
    .acts .btn {
      justify-content: center;
      border-color: var(--border);
    }
  }
</style>
