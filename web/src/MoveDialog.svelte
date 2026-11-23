<script lang="ts">
  // "Move to…": the keyboard and touch way to do what dragging a row onto a folder does.
  import { onMount } from 'svelte'
  import { composing } from './lib/keys'
  import { listNames } from './lib/filesapi'
  import { files } from './lib/filestore.svelte'
  import { mover, moveInto } from './lib/filetransfer.svelte'
  import { folderList, targets } from './lib/movepick'
  import { baseName, parentPath } from './lib/paths'
  import { app } from './lib/state.svelte'

  const project = app.currentId
  const from = mover.from
  let dialog: HTMLDialogElement | undefined = $state()
  let field: HTMLInputElement | undefined = $state()
  let query = $state(parentPath(from))
  let typed = $state(false)
  let active = $state(0)
  let paths = $state<string[]>([])
  let failed = $state(false)
  let restore: HTMLElement | null = null

  const loaded = $derived(
    Object.entries(files.dirs).flatMap(([k, d]) => {
      if (!k.startsWith(project + ':')) return []
      const dir = k.slice(project.length + 1)
      return d.entries.filter((e) => e.kind === 'dir').map((e) => (dir ? `${dir}/${e.name}` : e.name))
    }),
  )
  const rows = $derived(targets(folderList(paths, loaded), from, typed ? query : ''))
  const current = $derived(Math.min(active, Math.max(rows.length - 1, 0)))
  const label = (d: string) => (d === '' ? (app.current?.name ?? 'Project folder') : d)

  onMount(() => {
    restore = document.activeElement as HTMLElement | null
    const ctl = new AbortController()
    listNames(project, undefined, ctl.signal).then(
      (r) => (paths = r.paths),
      () => !ctl.signal.aborted && (failed = true),
    )
    field?.select()
    return () => ctl.abort()
  })

  const place = (n: HTMLDialogElement) => n.showModal()

  function done() {
    mover.from = ''
    ;(restore?.isConnected ? restore : document.querySelector<HTMLElement>('[role="tree"] button.main[tabindex="0"]'))?.focus()
  }

  function go(dir: string | undefined) {
    if (dir === undefined || app.currentId !== project) return
    dialog?.close()
    void moveInto(from, dir)
  }

  function keys(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (rows.length) active = (current + (e.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length
    } else if (e.key === 'Enter' && !composing(e)) {
      e.preventDefault()
      go(rows[current])
    }
  }

  $effect(() => {
    document.getElementById('move-' + current)?.scrollIntoView({ block: 'nearest' })
  })
</script>

<dialog use:place bind:this={dialog} class="dlg" aria-label={`Move ${baseName(from)}`} onclose={done} onclick={(e) => e.target === dialog && dialog.close()}>
  <h2>Move {baseName(from)} to</h2>
  <input
    bind:this={field}
    class="q"
    type="text"
    role="combobox"
    aria-expanded="true"
    aria-controls="move-list"
    aria-activedescendant={rows.length ? 'move-' + current : undefined}
    aria-label="Folder"
    placeholder="Type part of a folder name"
    autocomplete="off"
    spellcheck="false"
    bind:value={query}
    oninput={() => ((typed = true), (active = 0))}
    onkeydown={keys}
  />
  <p class="status" role="status">{failed ? 'Could not list every folder. Folders you opened in the tree are shown.' : ''}</p>
  <ul id="move-list" role="listbox" aria-label="Folders">
    {#each rows as d, i (d)}
      <!-- The input handles the keys; the rows only need the pointer. -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <li id={'move-' + i} role="option" aria-selected={i === current} onclick={() => go(d)} onpointermove={() => (active = i)}>{label(d)}</li>
    {:else}
      <li class="none" role="presentation">No folder matches “{query}”.</li>
    {/each}
  </ul>
  <footer>
    <button type="button" class="btn" onclick={() => dialog?.close()}>Cancel</button>
  </footer>
</dialog>

<style>
  h2 {
    margin: 0;
    padding: var(--s-3) var(--s-3) var(--s-2);
    font-size: var(--fs-m);
    overflow-wrap: anywhere;
  }
  .q {
    width: 100%;
    min-height: calc(var(--control-h) + var(--s-2));
    padding: 0 var(--s-3);
    border: 0;
    border-block: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
    font: inherit;
  }
  .q:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .status {
    margin: 0;
    padding: 0 var(--s-3);
    min-height: var(--s-3);
    font-size: var(--fs-s);
    color: var(--muted);
  }
  ul {
    max-height: 40dvh;
    margin: 0;
    padding: 0 var(--s-1) var(--s-1);
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    min-height: var(--row-h);
    padding: var(--s-1) var(--s-2);
    border-radius: var(--r-s);
    overflow-wrap: anywhere;
    cursor: pointer;
  }
  li[aria-selected='true'] {
    background: var(--selected);
  }
  .none {
    color: var(--muted);
    cursor: default;
  }
  @media (pointer: coarse) {
    li,
    footer .btn {
      min-height: 44px;
    }
  }
</style>
