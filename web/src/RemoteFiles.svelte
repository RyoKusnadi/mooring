<script lang="ts">
  // Browse a folder on an SSH host: open, upload, download, rename, delete, new folder. Paths are relative to the host's root.
  import { onMount, tick } from 'svelte'
  import { parentPath, joinPath, nameProblem } from './lib/paths'
  import { FileError, type Entry } from './lib/filesapi'
  import { crumbs, sortEntries, uploadMessage, uploadProblem } from './lib/remotedir'
  import { listRemote, uploadFile } from './lib/sshapi'
  import { ssh } from './lib/sshstore.svelte'
  import { CommandError } from './lib/events'
  import RemoteRow from './RemoteRow.svelte'

  let { conn, host }: { conn: string; host: string } = $props()

  let dir = $state('')
  let entries = $state<Entry[]>([])
  let truncated = $state(false)
  let error = $state('')
  let notice = $state('')
  let loading = $state(false)
  let adding = $state(false)
  let folderName = $state('')
  let over = $state(false)
  let input: HTMLInputElement | undefined = $state()
  let toggle: HTMLButtonElement | undefined = $state()
  let seq = 0

  const info = $derived(ssh.host(host))
  const trail = $derived(crumbs(dir, info?.name ?? 'Root'))
  const rows = $derived(sortEntries(entries))

  async function load(path = dir) {
    const mine = ++seq
    loading = true
    error = ''
    try {
      const l = await listRemote(conn, path)
      if (mine !== seq) return
      dir = path
      entries = l.entries
      truncated = l.truncated
    } catch (e) {
      if (mine === seq) error = e instanceof Error ? e.message : 'Could not list this folder.'
    } finally {
      if (mine === seq) loading = false
    }
  }

  onMount(() => void load(''))

  function changed(message?: string) {
    error = ''
    notice = message ?? ''
    void load()
  }

  async function toggleAdding() {
    adding = !adding
    if (adding) {
      await tick()
      document.getElementById('mk-name')?.focus()
    }
  }

  async function closeAdding() {
    adding = false
    await tick()
    toggle?.focus()
  }

  async function mkdir(e: SubmitEvent) {
    e.preventDefault()
    notice = ''
    const n = folderName.trim()
    error = nameProblem(n)
    if (error) return
    try {
      await ssh.cmd('ssh.mkdir', { conn, path: joinPath(dir, n) })
      adding = false
      folderName = ''
      changed(`Created folder ${n}.`)
    } catch (err) {
      error = err instanceof Error ? err.message : 'The folder was not created.'
    }
  }

  async function upload(files: FileList | File[]) {
    notice = ''
    const done: string[] = []
    const failed: string[] = []
    for (const f of Array.from(files)) {
      const p = uploadProblem(f.name, f.size)
      if (p) {
        failed.push(p)
        continue
      }
      try {
        await uploadFile(conn, joinPath(dir, f.name), f)
        done.push(f.name)
      } catch (e) {
        const code = e instanceof CommandError || e instanceof FileError ? e.code : ''
        failed.push(uploadMessage(f.name, code, e instanceof Error ? e.message : 'not uploaded'))
      }
    }
    error = failed.join(' ')
    notice = done.length ? `Uploaded ${done.join(', ')}.` : ''
    if (input) input.value = ''
    await load()
  }

  function drop(e: DragEvent) {
    e.preventDefault()
    over = false
    if (e.dataTransfer?.files.length) void upload(e.dataTransfer.files)
  }
</script>

<section
  class="wrap"
  class:over
  data-remote-view
  aria-label="Files on {info?.name ?? 'host'}"
  ondragover={(e) => {
    e.preventDefault()
    over = true
  }}
  ondragleave={() => (over = false)}
  ondrop={drop}
>
  <header>
    <nav aria-label="Folder">
      {#each trail as c, i (c.path)}
        {#if i > 0}<span aria-hidden="true">/</span>{/if}
        <button type="button" class="crumb" aria-current={i === trail.length - 1 ? 'page' : undefined} onclick={() => load(c.path)}>{c.label}</button>
      {/each}
    </nav>
    <div class="tools">
      {#if dir}<button type="button" class="btn" onclick={() => load(parentPath(dir))}>Up</button>{/if}
      <button type="button" class="btn" onclick={() => load()}>Refresh</button>
      <button type="button" class="btn" bind:this={toggle} onclick={toggleAdding} aria-expanded={adding}>New folder</button>
      <button type="button" class="btn primary" onclick={() => input?.click()}>Upload</button>
      <input bind:this={input} class="sr" type="file" multiple tabindex="-1" aria-label="Choose files to upload" onchange={(e) => e.currentTarget.files && upload(e.currentTarget.files)} />
    </div>
  </header>
  {#if adding}
    <form class="mk" onsubmit={mkdir}>
      <label class="sr" for="mk-name">Folder name</label>
      <input id="mk-name" class="input mono" placeholder="Folder name" bind:value={folderName} autocomplete="off" spellcheck="false" onkeydown={(e) => e.key === 'Escape' && (e.stopPropagation(), closeAdding())} />
      <button type="submit" class="btn">Create</button>
    </form>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if notice}<p class="notice" role="status">{notice}</p>{/if}
  {#if loading && entries.length === 0}
    <p class="muted" role="status">Loading…</p>
  {:else if rows.length === 0 && !error}
    <p class="muted">This folder is empty. Upload a file or drop one here.</p>
  {:else}
    <ul>
      {#each rows as entry (entry.name)}
        <RemoteRow {entry} {dir} {conn} {host} onOpenDir={(p) => load(p)} onChanged={changed} />
      {/each}
    </ul>
    {#if truncated}<p class="muted">Only the first entries are shown. Open a subfolder to see the rest.</p>{/if}
  {/if}
  {#if over}<div class="drop" aria-hidden="true">Drop files to upload to {dir || 'the root'}</div>{/if}
</section>

<style>
  .wrap {
    position: relative;
    min-height: 100%;
    padding: var(--s-3);
  }
  .wrap.over {
    outline: 2px dashed var(--accent);
    outline-offset: -4px;
  }
  header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    margin-bottom: var(--s-2);
  }
  nav {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    min-width: 0;
    color: var(--muted);
  }
  .crumb {
    min-height: var(--control-h);
    padding: 0 var(--s-1);
    border: 0;
    background: none;
    color: var(--fg);
    font: inherit;
    font-family: var(--font-mono);
  }
  .crumb[aria-current='page'] {
    font-weight: var(--fw-semibold);
  }
  .tools {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
  }
  .mk {
    display: flex;
    gap: var(--s-2);
    margin-bottom: var(--s-2);
  }
  .mk .input {
    flex: 1;
    min-width: 0;
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
  .muted,
  .notice {
    margin: var(--s-2) 0;
    color: var(--muted);
  }
  .drop {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background: var(--bg);
    opacity: 0.9;
    pointer-events: none;
  }
</style>
