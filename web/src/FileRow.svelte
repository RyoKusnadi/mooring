<script lang="ts">
  import { tick } from 'svelte'
  import FileMenu from './FileMenu.svelte'
  import FileRow from './FileRow.svelte'
  import NameRow from './NameRow.svelte'
  import { actions } from './lib/actions'
  import { plainError, type Entry } from './lib/filesapi'
  import { files } from './lib/filestore.svelte'
  import { dropInto, mover, transferItems } from './lib/filetransfer.svelte'
  import { joinPath, parentPath } from './lib/paths'
  import { acceptDrag, endDrag, readDrop, startDrag } from './lib/treedrag'
  import { treeKeys } from './lib/treekeys'

  let { project, dir, entry, depth, onNavigate, onError }: { project: string; dir: string; entry: Entry; depth: number; onNavigate: () => void; onError: (m: string) => void } = $props()

  const path = $derived(joinPath(dir, entry.name))
  const isDir = $derived(entry.kind === 'dir')
  const usable = $derived(entry.kind === 'dir' || entry.kind === 'file')
  const open = $derived(isDir && files.isOpen(project, path))
  const child = $derived(files.dir(project, path))
  const dim = $derived(entry.hidden || entry.ignored || !usable)
  const note = $derived(entry.unsafe === 'outside' ? 'link leaves the project' : entry.unsafe === 'broken' ? 'broken link' : entry.ignored ? 'ignored' : entry.hidden ? 'hidden' : '')
  const op = $derived(files.editing?.path === path ? files.editing.op : null)
  let menu: FileMenu | undefined = $state()
  let paying = $state(false)

  /** A mouse click previews the file in a tab the next click replaces; Enter opens it for keeps. */
  function pick(e: MouseEvent) {
    if (isDir) {
      files.target = path
      files.toggle(project, path)
    } else {
      files.target = dir
      actions.openFile(path, undefined, e.detail > 0)
      onNavigate()
    }
  }

  const ask = (o: 'new-file' | 'new-dir' | 'rename' | 'delete') => (files.editing = { op: o, path })

  function startNew(kind: 'new-file' | 'new-dir') {
    files.expand(project, path)
    ask(kind)
  }

  const items = $derived([
    ...(isDir ? [{ label: 'New file here', run: () => startNew('new-file') }, { label: 'New folder here', run: () => startNew('new-dir') }] : []),
    ...transferItems(path, isDir),
    { label: 'Rename…', run: () => ask('rename') },
    { label: 'Delete…', run: () => ask('delete'), danger: true },
  ])

  // A folder row takes drops; a file row refuses them without letting the empty space below take them.
  let over = $state(false)
  const dragOver = (e: DragEvent) => (isDir ? (over = acceptDrag(e, path)) : e.stopPropagation())
  function drop(e: DragEvent) {
    over = false
    if (!isDir) return e.stopPropagation()
    const d = readDrop(e, path)
    dropInto(d, path)
  }

  async function remove(permanent: boolean) {
    paying = true
    try {
      await actions.deleteEntry(path, permanent)
      files.editing = null
    } catch (e) {
      const code = (e as { code?: string }).code
      if (code === 'TRASH_UNAVAILABLE') files.editing = { op: 'delete-forever', path }
      else {
        files.editing = null
        onError('Could not delete ' + entry.name + '. ' + plainError(e))
      }
    } finally {
      paying = false
    }
  }

  async function rename(name: string) {
    files.editing = null
    if (name === entry.name) return
    try {
      await actions.renameEntry(path, joinPath(parentPath(path), name))
    } catch (e) {
      onError('Could not rename ' + entry.name + '. ' + plainError(e))
    }
  }

  async function create(name: string) {
    const kind = files.editing?.op === 'new-dir' ? 'dir' : 'file'
    files.editing = null
    try {
      const made = await files.create(project, path, name, kind)
      if (kind === 'file') actions.openFile(made)
    } catch (e) {
      onError('Could not create ' + name + '. ' + plainError(e))
    }
  }

  let confirmButton: HTMLButtonElement | undefined = $state()
  let keepButton: HTMLButtonElement | undefined = $state()
  // A move to the trash can be undone, so it starts on the action; deleting for good starts on Keep.
  $effect(() => {
    if (op === 'delete') void tick().then(() => confirmButton?.focus())
    else if (op === 'delete-forever') void tick().then(() => keepButton?.focus())
  })

  // Esc keeps the item, like the Keep button.
  function keepOnEscape(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.stopPropagation()
    files.editing = null
  }
</script>

<li role="treeitem" aria-selected="false" aria-expanded={isDir ? open : undefined}>
  {#if op === 'rename'}
    <NameRow {depth} label={`New name for ${entry.name}`} initial={entry.name} onDone={rename} onCancel={() => (files.editing = null)} />
  {:else if op === 'delete' || op === 'delete-forever'}
    <div class="ask" style:--depth={depth} role="group" aria-label={`Delete ${entry.name}`}>
      <p>
        {#if op === 'delete'}Move {entry.name}{isDir ? ' and what is in it' : ''} to the trash?{:else}There is no trash to move it to. Delete {entry.name}{isDir ? ' and what is in it' : ''} for good? This cannot be undone.{/if}
      </p>
      <div class="buttons">
        <button type="button" bind:this={confirmButton} class="btn danger small" disabled={paying} onkeydown={keepOnEscape} onclick={() => remove(op === 'delete-forever')}>{op === 'delete' ? 'Move to trash' : 'Delete permanently'}</button>
        <button type="button" bind:this={keepButton} class="btn small" onkeydown={keepOnEscape} onclick={() => (files.editing = null)}>Keep</button>
      </div>
    </div>
  {:else}
    <div class="row" class:dim class:over role="presentation" oncontextmenu={(e) => usable && (e.preventDefault(), menu?.open())} ondragover={dragOver} ondragleave={() => (over = false)} ondrop={drop}>
      <button
        type="button"
        class="main"
        style:--depth={depth}
        title={path + (note ? ` (${note})` : '')}
        data-path={path}
        tabindex={files.cursor === path ? 0 : -1}
        disabled={!usable}
        onclick={pick}
        ondblclick={() => !isDir && actions.keepFile(path)}
        onfocus={() => (files.cursor = path)}
        draggable={usable}
        ondragstart={(e) => startDrag(e, path)}
        ondragend={endDrag}
        onkeydown={(e) => treeKeys(e, { isDir, open, usable, toggle: () => files.toggle(project, path), rename: () => ask('rename'), remove: () => ask('delete'), move: () => (mover.from = path) })}
      >
        <span class="chev" aria-hidden="true">{isDir ? (open ? '▾' : '▸') : ''}</span>
        <span class="name">{entry.name}</span>
        {#if note}<span class="sr">({note})</span>{/if}
      </button>
      {#if usable}<FileMenu bind:this={menu} label={`Actions for ${entry.name}`} {items} />{/if}
    </div>
  {/if}
  {#if isDir && open}
    <ul role="group">
      {#if files.editing && (files.editing.op === 'new-file' || files.editing.op === 'new-dir') && files.editing.path === path}
        <li><NameRow depth={depth + 1} label={files.editing.op === 'new-dir' ? 'Name of the new folder' : 'Name of the new file'} initial="" onDone={create} onCancel={() => (files.editing = null)} /></li>
      {/if}
      {#if child?.state === 'loading' && child.entries.length === 0}
        <li class="hint" style:--depth={depth + 1} role="status">Loading…</li>
      {:else if child?.state === 'error'}
        <li class="hint" style:--depth={depth + 1} role="alert">{child.error} <button type="button" class="link" onclick={() => files.load(project, path)}>Try again</button></li>
      {:else if child}
        {#each child.entries as e (e.name)}
          <FileRow {project} dir={path} entry={e} depth={depth + 1} {onNavigate} {onError} />
        {:else}
          <li class="hint" style:--depth={depth + 1}>Empty folder.</li>
        {/each}
        {#if child.truncated}<li class="hint" style:--depth={depth + 1}>Showing {child.entries.length} of {child.total}.</li>{/if}
      {/if}
    </ul>
  {/if}
</li>

<style>
  li {
    list-style: none;
  }
  ul {
    margin: 0;
    padding: 0;
  }
  .row {
    display: flex;
    align-items: center;
    min-height: var(--control-h);
    border-radius: var(--r-s);
    transition: background-color var(--ease);
  }
  .row:hover {
    background: var(--hover);
  }
  .row.over {
    background: var(--hover);
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .row:hover :global(.more),
  .row:focus-within :global(.more) {
    opacity: 1;
  }
  .main {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    min-height: var(--control-h);
    padding: 0 var(--s-1) 0 calc(var(--s-1) + var(--depth, 0) * var(--s-3));
    border: 0;
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .dim .main {
    color: var(--muted);
  }
  .main:disabled {
    cursor: not-allowed;
  }
  .chev {
    flex: none;
    width: 12px;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    padding: var(--s-1) var(--s-2) var(--s-1) calc(var(--s-1) + 12px + var(--s-1) + var(--depth, 0) * var(--s-3));
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
  }
  .ask {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) 0;
    padding: var(--s-2) var(--s-2) var(--s-2) calc(var(--s-2) + var(--depth, 0) * var(--s-3));
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--raised);
  }
  .ask p {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .small {
    padding: 0 var(--s-2);
  }
</style>
