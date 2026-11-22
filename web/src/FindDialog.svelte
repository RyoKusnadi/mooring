<script lang="ts">
  import { composing, isMac } from './lib/keys'
  import { onMount } from 'svelte'
  import { actions } from './lib/actions'
  import { FileError, listNames, plainError, search, type Match } from './lib/filesapi'
  import { files } from './lib/filestore.svelte'
  import { rankPaths } from './lib/fuzzy'
  import { cached, remember } from './lib/quickindex'
  import { recentFiles } from './lib/recentfiles'
  import FindList from './FindList.svelte'
  import FindOptions from './FindOptions.svelte'
  import { app } from './lib/state.svelte'

  const project = app.currentId
  let dialog: HTMLDialogElement | undefined = $state()
  let mode = $state<'file' | 'text'>(files.finder ?? 'file')
  let query = $state(files.finderQuery)
  let active = $state(0)
  let regex = $state(false)
  let matchCase = $state(false)
  let glob = $state('')
  let replacing = $state(false)
  let stage = $state<'edit' | 'preview' | 'done'>('edit')
  let repBusy = $state(false)
  // The plan was made for this search; it stays fixed until the user goes back.
  const frozen = $derived(stage !== 'edit' || repBusy)

  // Open file by name
  let paths = $state<string[]>(cached(project)?.paths ?? [])
  let indexing = $state(false)
  let incomplete = $state(false)
  let namesFailed = $state(false)
  let attempt = $state(0)
  // Search in files
  let hits = $state<Match[]>([])
  const hitPaths = $derived([...new Set(hits.map((h) => h.path))])
  let phase = $state<'idle' | 'searching' | 'done' | 'cut' | 'error'>('idle')
  let problem = $state('')
  let seenTick = $state(0)
  let engine = new AbortController()
  let restore = () => {}
  let timer: ReturnType<typeof setTimeout> | undefined

  const shownPaths = $derived(rankPaths(paths, query, 50, recentFiles(project)))
  const rows = $derived(
    mode === 'file'
      ? shownPaths.map((p) => ({ id: 'f-' + p, path: p, line: 1, match: null as Match | null }))
      : hits.map((m, i) => ({ id: `m-${i}`, path: m.path, line: m.line, match: m as Match | null })),
  )
  const current = $derived(Math.min(active, Math.max(rows.length - 1, 0)))

  onMount(() => {
    const before = document.activeElement as HTMLElement | null
    restore = () => (before?.isConnected ? before : document.querySelector<HTMLElement>('[role="tree"] button.main[tabindex="0"], button[aria-label="Add project"]'))?.focus()
    return () => {
      engine.abort()
      clearTimeout(timer)
    }
  })

  // The index builds when the dialog opens in file mode; a recent one is reused and refreshed.
  $effect(() => {
    if (mode !== 'file') return
    void attempt // "Try again" reruns this
    const fresh = cached(project)
    if (fresh?.complete) return
    const ctl = new AbortController()
    indexing = true
    namesFailed = false
    listNames(project, undefined, ctl.signal).then(
      (r) => {
        if (ctl.signal.aborted) return
        const index = { paths: r.paths, complete: !r.truncated }
        remember(project, index)
        paths = index.paths
        incomplete = !index.complete
        indexing = false
      },
      () => {
        if (ctl.signal.aborted) return
        namesFailed = true
        indexing = false
      },
    )
    return () => {
      ctl.abort()
      indexing = false
    }
  })

  function run() {
    engine.abort()
    clearTimeout(timer)
    hits = []
    problem = ''
    if (mode !== 'text' || query === '') return void (phase = 'idle')
    phase = 'searching'
    seenTick = files.changeTick
    timer = setTimeout(go, 200)
  }

  async function go() {
    engine = new AbortController()
    const mine = engine
    try {
      const done = await search(project, { q: query, regex, matchCase, glob: glob.trim() }, (m) => void (mine.signal.aborted || hits.push(m)), mine.signal)
      if (mine.signal.aborted) return
      phase = done?.truncated || done?.reason === 'timeout' ? 'cut' : 'done'
    } catch (e) {
      if (mine.signal.aborted) return
      phase = 'error'
      problem = regex && e instanceof FileError && e.status === 400 ? 'That is not a valid regular expression. Fix it or turn off Regex.' : plainError(e, 'The search could not run. Try again.')
    }
  }

  // A close event can arrive after the dialog was swapped out (Alt+O over the results panel); it must not clear the new one.
  function onclosed() {
    if (!dialog?.isConnected) return
    files.finder = null
    files.finderQuery = ''
    restore()
  }

  function setMode(m: 'file' | 'text') {
    mode = m
    active = 0
    run()
  }

  // Finding a file by name is a modal question. Search results are a panel beside the editor: it stays
  // while files open, so the user can walk through the hits, and closes with Esc or its button.
  function place(node: HTMLDialogElement) {
    if (mode === 'text') node.show()
    else node.showModal()
  }

  function open(row: { path: string; line: number } | undefined) {
    if (!row || app.currentId !== project) return
    if (mode === 'file') dialog?.close()
    actions.openFile(row.path, mode === 'text' ? row.line : undefined)
  }

  // Alt+R in the panel opens the replace field, or goes to it.
  function openReplace() {
    if (replacing) document.getElementById('find-replace')?.focus()
    else replacing = true
  }

  function keys(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (rows.length) active = (current + (e.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length
    } else if (e.key === 'Enter' && !composing(e)) {
      e.preventDefault()
      open(rows[current])
    }
  }

  $effect(() => {
    document.getElementById('find-' + rows[current]?.id)?.scrollIntoView({ block: 'nearest' })
  })

  // The results belong to the project they searched. Leaving it closes the panel, so a hit never opens elsewhere.
  $effect(() => {
    if (app.currentId !== project) dialog?.close()
  })
  const stale = $derived(mode === 'text' && (phase === 'done' || phase === 'cut') && files.changeTick !== seenTick)

  const summary = $derived(
    phase === 'searching' ? 'Searching…' :
    phase === 'cut' ? `Showing the first ${hits.length} lines. Narrow the search to see the rest.` :
    phase === 'done' ? (hits.length === 0 ? `Nothing found for “${query}”.` : `${hits.length} ${hits.length === 1 ? 'line' : 'lines'}`) : '',
  )
</script>

{#key mode}
<!-- svelte-ignore a11y_no_noninteractive_element_interactions: Escape closes the search panel from anywhere in it -->
<dialog use:place bind:this={dialog} class="dlg wide" class:panel={mode === 'text'} aria-label="Find in project" onkeydown={(e) => (e.key === 'Escape' && mode === 'text' ? (e.preventDefault(), dialog?.close()) : e.altKey && e.code === 'KeyR' && mode === 'text' && !(isMac && (e.target as HTMLElement).id === 'find-q') && (e.preventDefault(), openReplace()))} onclose={onclosed} onclick={(e) => mode === 'file' && e.target === dialog && dialog.close()}>
  <div class="modes" role="group" aria-label="What to find">
    <button type="button" class="btn ghost" aria-pressed={mode === 'file'} onclick={() => setMode('file')}>Open file</button>
    <button type="button" class="btn ghost" aria-pressed={mode === 'text'} onclick={() => setMode('text')}>Search text</button>
    {#if mode === 'text'}<button type="button" class="btn ghost shut" onclick={() => dialog?.close()}>Close</button>{/if}
  </div>
  <!-- svelte-ignore a11y_autofocus -->
  <input
    class="q"
    id="find-q"
    type="text"
    role="combobox"
    aria-expanded="true"
    aria-controls="find-list"
    aria-activedescendant={rows[current] ? 'find-' + rows[current].id : undefined}
    aria-label={mode === 'file' ? 'File name' : 'Text to find'}
    placeholder={mode === 'file' ? 'Type part of a file name' : 'Type text to find in this project'}
    autocomplete="off"
    spellcheck="false"
    autofocus
    bind:value={query}
    disabled={replacing && frozen}
    oninput={() => ((active = 0), run())}
    onkeydown={keys}
  />
  {#if mode === 'text'}<FindOptions bind:matchCase bind:regex bind:glob disabled={replacing && frozen} onChange={run} />
    <div class="repbar"><button type="button" class="btn ghost" aria-expanded={replacing} aria-controls="find-replace-box" disabled={repBusy} title="Replace in files (Alt+R)" onclick={() => ((replacing = !replacing), (stage = 'edit'))}>Replace…</button></div>
    <div id="find-replace-box">
      {#if replacing}{#await import('./FindReplace.svelte') then m}<m.default {project} {query} {regex} {matchCase} paths={hitPaths} {phase} bind:stage bind:busy={repBusy} onApplied={run} />{/await}{/if}
    </div>
  {/if}
  <p class="status" role="status">
    {#if mode === 'file'}{indexing ? 'Listing files…' : incomplete ? 'This project has more files than can be listed, so some names are missing. Search text searches all of them.' : ''}{:else}{summary}{#if stale}{' '}Files changed since this search. <button type="button" class="btn small" onclick={run}>Refresh results</button>{/if}{/if}
  </p>
  {#if mode === 'file' && namesFailed}
    <p class="error" role="alert">Could not list the files. Check that the server is running. <button type="button" class="btn small" onclick={() => attempt++}>Try again</button></p>
  {/if}
  {#if problem}<p class="error" role="alert">{problem}</p>{/if}
  {#if stage === 'edit' || !replacing}<FindList {rows} {current} {mode} {query} {indexing} onopen={open} onhover={(i) => (active = i)} />{/if}
</dialog>
{/key}

<style>
  .modes {
    display: flex;
    gap: var(--s-1);
    padding: var(--s-2) var(--s-2) 0;
  }
  .repbar {
    padding: var(--s-1) var(--s-2) 0;
  }
  .shut {
    margin-left: auto;
  }
  dialog.panel {
    position: fixed;
    inset: 48px var(--s-3) auto auto;
    width: min(520px, calc(100vw - var(--s-3) * 2));
    max-height: calc(100dvh - 72px);
    margin: 0;
  }
  .modes [aria-pressed='true'] {
    background: var(--selected);
  }
  .q {
    width: 100%;
    min-height: calc(var(--control-h) + var(--s-2));
    padding: 0 var(--s-3);
    border: 0;
    border-bottom: 1px solid var(--border);
    background: transparent;
    color: var(--fg);
    font: inherit;
  }
  .q:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
    border-bottom-color: transparent;
  }
  .status,
  .error {
    margin: 0;
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-s);
  }
  .status {
    min-height: calc(var(--fs-s) * 1.5 + var(--s-2));
    color: var(--muted);
  }
</style>
