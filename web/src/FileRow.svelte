<script lang="ts">
  import FileAsk from './FileAsk.svelte'
  import FileMenu from './FileMenu.svelte'
  import FileRow from './FileRow.svelte'
  import Icon from './Icon.svelte'
  import NameRow from './NameRow.svelte'
  import { actions } from './lib/actions'
  import { plainError, type Entry } from './lib/filesapi'
  import { files } from './lib/filestore.svelte'
  import { dropInto, mover, transferItems } from './lib/filetransfer.svelte'
  import { joinPath, parentPath } from './lib/paths'
  import { acceptDrag, endDrag, readDrop, startDrag } from './lib/treedrag'
  import { treeFilter } from './lib/treefilter.svelte'
  import { treeKeys } from './lib/treekeys'
  import { letterTone, letterWord } from './lib/treemarks'
  import { treeMarks } from './lib/treemarks.svelte'

  let { project, dir, entry, depth, onNavigate, onError, inherited = false }: { project: string; dir: string; entry: Entry; depth: number; onNavigate: () => void; onError: (m: string) => void; inherited?: boolean } = $props()

  const path = $derived(joinPath(dir, entry.name))
  const isDir = $derived(entry.kind === 'dir')
  const usable = $derived(entry.kind === 'dir' || entry.kind === 'file')
  const forced = $derived(treeFilter.forced(project, dir, entry))
  const shown = $derived(treeFilter.shows(project, dir, entry, inherited))
  const open = $derived(isDir && (files.isOpen(project, path) || forced))
  const letter = $derived(isDir ? '' : (treeMarks.current.files.get(path) ?? ''))
  const holdsChanges = $derived(isDir && treeMarks.current.dirs.has(path))
  const child = $derived(files.dir(project, path))
  const dim = $derived(entry.hidden || entry.ignored || !usable)
  const note = $derived(entry.unsafe === 'outside' ? 'link leaves the project' : entry.unsafe === 'broken' ? 'broken link' : entry.ignored ? 'ignored' : entry.hidden ? 'hidden' : '')
  const op = $derived(files.editing?.path === path ? files.editing.op : null)
  let menu: FileMenu | undefined = $state()

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
</script>

{#if shown}
<li role="treeitem" aria-selected="false" aria-expanded={isDir ? open : undefined}>
  {#if op === 'rename'}
    <NameRow {depth} label={`New name for ${entry.name}`} initial={entry.name} onDone={rename} onCancel={() => (files.editing = null)} />
  {:else if op === 'delete' || op === 'delete-forever'}
    <FileAsk {entry} {path} {isDir} {depth} {op} {onError} />
  {:else}
    <div class="row" class:dim class:over data-tone={letter ? letterTone(letter) : undefined} role="presentation" oncontextmenu={(e) => usable && (e.preventDefault(), menu?.open())} ondragover={dragOver} ondragleave={() => (over = false)} ondrop={drop}>
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
        <span class="chev" class:open aria-hidden="true">{#if isDir}<Icon name="chevronRight" />{/if}</span>
        <span class="ico" aria-hidden="true"><Icon name={isDir ? (open ? 'folderOpen' : 'folder') : 'file'} /></span>
        <span class="name" class:hit={treeFilter.matched(entry)}>{entry.name}</span>
        {#if note}<span class="sr">({note})</span>{/if}
      </button>
      {#if letter}<span class="mark" role="img" aria-label={letterWord(letter)} title={letterWord(letter)}>{letter}</span>{:else if holdsChanges}<span class="dot" role="img" aria-label="Contains changes" title="Contains changes"></span>{/if}
      {#if usable}<FileMenu bind:this={menu} label={`Actions for ${entry.name}`} {items} />{/if}
    </div>
  {/if}
  {#if isDir && open}
    <ul role="group" class="group" style:--depth={depth}>
      {#if files.editing && (files.editing.op === 'new-file' || files.editing.op === 'new-dir') && files.editing.path === path}
        <li><NameRow depth={depth + 1} label={files.editing.op === 'new-dir' ? 'Name of the new folder' : 'Name of the new file'} initial="" onDone={create} onCancel={() => (files.editing = null)} /></li>
      {/if}
      {#if child?.state === 'loading' && child.entries.length === 0}
        <li class="hint" style:--depth={depth + 1} role="status">Loading…</li>
      {:else if child?.state === 'error'}
        <li class="hint" style:--depth={depth + 1} role="alert">{child.error} <button type="button" class="link" onclick={() => files.load(project, path)}>Try again</button></li>
      {:else if child}
        {#each child.entries as e (e.name)}
          <FileRow {project} dir={path} entry={e} depth={depth + 1} {onNavigate} {onError} inherited={inherited || treeFilter.matched(entry)} />
        {:else}
          <li class="hint" style:--depth={depth + 1}>Empty folder.</li>
        {/each}
        {#if child.truncated}<li class="hint" style:--depth={depth + 1}>Showing {child.entries.length} of {child.total}.</li>{/if}
      {/if}
    </ul>
  {/if}
</li>
{/if}

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
    min-height: var(--row-h);
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
    min-height: var(--row-h);
    padding: 0 var(--s-1) 0 calc(var(--s-1) + var(--depth, 0) * var(--s-4));
    border: 0;
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .dim .main,
  .dim .ico {
    color: var(--faint);
  }
  .dim .ico,
  .dim .chev {
    opacity: 0.75;
  }
  .main:disabled {
    cursor: not-allowed;
  }
  .chev,
  .ico {
    display: inline-flex;
    flex: none;
    width: 16px;
    color: var(--muted);
  }
  .chev :global(svg) {
    transition: transform var(--ease);
  }
  .chev.open :global(svg) {
    transform: rotate(90deg);
  }
  /* A thin line under the parent's chevron, so a deep folder shows which one it belongs to. */
  .group {
    position: relative;
  }
  .group::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: calc(var(--s-1) + var(--depth, 0) * var(--s-4) + 8px);
    width: 1px;
    background: var(--border);
    pointer-events: none;
  }
  .name.hit {
    text-decoration: underline;
    text-decoration-color: var(--accent);
    text-underline-offset: 3px;
  }
  .mark {
    flex: none;
    width: 16px;
    margin-right: var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    text-align: center;
  }
  .dot {
    flex: none;
    width: 6px;
    height: 6px;
    margin: 0 var(--s-2) 0 var(--s-1);
    border-radius: 50%;
    background: var(--muted);
  }
  [data-tone='new'] .name,
  [data-tone='new'] .mark {
    color: var(--success);
  }
  [data-tone='edit'] .mark {
    color: var(--fg);
  }
  [data-tone='gone'] .name,
  [data-tone='gone'] .mark {
    color: var(--danger);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    padding: var(--s-1) var(--s-2) var(--s-1) calc(var(--s-1) * 3 + 32px + var(--depth, 0) * var(--s-4));
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
</style>
