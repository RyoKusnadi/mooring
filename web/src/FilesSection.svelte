<script lang="ts">
  import Skeleton from './Skeleton.svelte'
  import { tick, untrack } from 'svelte'
  import FileNote from './FileNote.svelte'
  import FileRow from './FileRow.svelte'
  import Icon from './Icon.svelte'
  import MoveDialog from './MoveDialog.svelte'
  import { dropInto, mover } from './lib/filetransfer.svelte'
  import { acceptDrag, readDrop } from './lib/treedrag'
  import { treeFilter } from './lib/treefilter.svelte'
  import NameRow from './NameRow.svelte'
  import { plainError } from './lib/filesapi'
  import { files } from './lib/filestore.svelte'
  import { git } from './lib/gitstore.svelte'
  import { app } from './lib/state.svelte'
  import { filePaths } from './lib/layout'
  import { scrollMore } from './lib/scrollmore'

  let { onNavigate, onAdd }: { onNavigate: () => void; onAdd: () => void } = $props()

  const project = $derived(app.currentId)
  const root = $derived(files.dir(project, ''))
  let error = $state('')
  // One tab stop for the whole tree: the last row used, or the first row if that row is gone.
  $effect(() => {
    const first = root?.entries.find((e) => treeFilter.shows(project, '', e))?.name
    const cursor = files.cursor
    void root?.entries.length
    void treeFilter.query
    if (!first) return
    void tick().then(() => {
      if (!document.querySelector(`[role="tree"] button.main[data-path="${CSS.escape(cursor)}"]`)) files.cursor = first
    })
  })
  let shown = ''

  // The root folder loads when the project is shown; folders load when opened.
  // Only the project and readiness are tracked: load() reads the folder state it writes.
  $effect(() => {
    const id = project
    const ready = app.ready
    untrack(() => {
      if (id !== shown) {
        shown = id
        files.target = ''
        files.editing = null
        error = ''
      }
      if (id && ready) void files.load(id, '')
    })
  })

  // Tell the server which folders and files this window shows, so it sends only those changes.
  const openFiles = $derived(app.layout ? filePaths(app.layout) : [])
  $effect(() => {
    if (!project || !app.ready) return
    // The folder Changes shows is watched too, so new and removed files there update the list.
    files.watch(project, [...new Set([...files.openDirs(project), ...(git.folder ? [git.folder] : [])])], openFiles)
  })
  $effect(() => {
    if (app.status === 'live') files.renew()
  })

  function start(op: 'new-file' | 'new-dir') {
    files.expand(project, files.target)
    files.editing = { op, path: files.target }
  }

  /** Esc in the filter clears it, and is not taken by anything behind it. */
  function filterKeys(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !treeFilter.query) return
    e.preventDefault()
    e.stopPropagation()
    treeFilter.query = ''
  }

  const found = $derived(treeFilter.active ? treeFilter.count(project) : 0)
  const none = $derived(treeFilter.active && !!root && !root.entries.some((e) => treeFilter.shows(project, '', e)))

  let over = $state(false)
  function drop(e: DragEvent) {
    over = false
    const d = readDrop(e, '')
    dropInto(d, '')
  }

  async function create(name: string) {
    const kind = files.editing?.op === 'new-dir' ? 'dir' : 'file'
    files.editing = null
    try {
      await files.create(project, '', name, kind)
    } catch (e) {
      error = 'Could not create ' + name + '. ' + plainError(e)
    }
  }
</script>

{#if app.ready && app.current}
  <section aria-label={`Files in ${app.current.name}`}>
    <div class="bar">
      <input class="input find" type="search" aria-label="Filter files" placeholder="Filter files" autocomplete="off" spellcheck="false" bind:value={treeFilter.query} onkeydown={filterKeys} />
      <button type="button" class="icon-btn" aria-label="New file" title="New file" onclick={() => start('new-file')}><Icon name="filePlus" /></button>
      <button type="button" class="icon-btn" aria-label="New folder" title="New folder" onclick={() => start('new-dir')}><Icon name="folderPlus" /></button>
      <button type="button" class="icon-btn" aria-label="Collapse all folders" title="Collapse all folders" onclick={() => files.collapseAll(project)}><Icon name="collapseAll" /></button>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <FileNote />
    {#if mover.from}{#key mover.from}<MoveDialog />{/key}{/if}
    {#if !root || (root.state === 'loading' && root.entries.length === 0)}
      <div class="hint" role="status"><Skeleton rows={4} label="Loading files" /></div>
    {:else if root.state === 'error' && root.missing}
      <div class="missing" role="alert">
        <p class="error">The folder of this project is missing: <code>{app.current.path}</code>. Put it back, or choose another folder.</p>
        <div class="row">
          <button type="button" class="btn" onclick={onAdd}>Choose another folder</button>
          <button type="button" class="btn danger solid" onclick={() => app.remove(project).catch((e: unknown) => (error = plainError(e)))}>Remove project</button>
        </div>
      </div>
    {:else if root.state === 'error'}
      <p class="error" role="alert">{root.error}{#if root.retry !== false} <button type="button" class="link" onclick={() => files.load(project, '')}>Try again</button>{/if}</p>
    {:else}
      <!-- Dropping on empty space means the project folder. -->
      <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
      <p class="sr" role="status">{treeFilter.active ? (found === 0 ? 'No files match' : found === 1 ? '1 match' : `${found} matches`) : ''}</p>
      <ul use:scrollMore role="tree" aria-label={`Files in ${app.current.name}`} class:over ondragover={(e) => (over = acceptDrag(e, ''))} ondragleave={() => (over = false)} ondrop={drop}>
        {#if files.editing && (files.editing.op === 'new-file' || files.editing.op === 'new-dir') && files.editing.path === ''}
          <li><NameRow depth={0} label={files.editing.op === 'new-dir' ? 'Name of the new folder' : 'Name of the new file'} initial="" onDone={create} onCancel={() => (files.editing = null)} /></li>
        {/if}
        {#each root.entries as entry (entry.name)}
          <FileRow {project} dir="" {entry} depth={0} {onNavigate} onError={(m) => (error = m)} />
        {:else}
          <li class="hint">This project folder is empty. Use New file to add one.</li>
        {/each}
        {#if none}<li class="hint">No loaded file or folder matches "{treeFilter.query.trim()}". Folders that are closed are not searched.</li>{/if}
        {#if root.truncated}<li class="hint">Showing {root.entries.length} of {root.total}.</li>{/if}
      </ul>
    {/if}
  </section>
{/if}

<style>
  section {
    display: flex;
    flex: 1 1 0;
    flex-direction: column;
    min-height: 120px;
  }
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    padding: var(--s-2) var(--s-2) var(--s-1);
  }
  .find {
    flex: 1;
    min-width: 0;
  }
  /* The list scrolls in its own area. The fade adds no height, so a short list never scrolls. */
  ul {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 2px var(--s-2) 0;
    overflow-y: auto;
    list-style: none;
  }
  /* The list fades into the sidebar at its bottom edge, only while more rows wait below. At the end the fade is gone, so the last row is whole. */
  ul:global([data-more])::after {
    content: '';
    display: block;
    position: sticky;
    bottom: 0;
    height: var(--s-4);
    margin-top: calc(-1 * var(--s-4));
    background: linear-gradient(to bottom, transparent, var(--surface));
    pointer-events: none;
  }
  ul.over {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .hint,
  .missing {
    display: grid;
    gap: var(--s-2);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
    padding: 0 var(--s-3);
  }
  .error {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-s);
  }
  .hint {
    color: var(--muted);
    list-style: none;
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
