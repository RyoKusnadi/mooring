<script lang="ts">
  import { onMount, tick, untrack } from 'svelte'
  import { actions } from './lib/actions'
  import { attach } from './lib/attachstore.svelte'
  import { FileDoc } from './lib/filedoc.svelte'
  import { files } from './lib/filestore.svelte'
  import { lsp } from './lib/lspstore.svelte'
  import { MAX_LSP_BYTES } from './lib/lspmatch'
  import type { SymbolStep } from './lib/lspsymbols'
  import Crumbs from './Crumbs.svelte'
  import LspBar from './LspBar.svelte'
  import FileRefactor from './FileRefactor.svelte'
  import FileBars from './FileBars.svelte'
  import FileBlame from './FileBlame.svelte'
  import { baseName, formatBytes } from './lib/paths'
  import { isImagePath, isPdfPath } from './lib/imagetype'
  import { previewKind, rememberPreview, wasPreview } from './lib/previewtype'
  import { ssh } from './lib/sshstore.svelte'
  import { tabHint } from './lib/tabhint.svelte'
  import { app } from './lib/state.svelte'

  /** `remote` is an SSH connection id: the file is on that host, and no language server runs for it. */
  let { id, path, remote = '' }: { id: string; path: string; remote?: string } = $props()

  // The view is keyed by tab and path, so these never change for one instance.
  const doc = untrack(() => new FileDoc(app.currentId, id, path, remote))
  let host: HTMLElement | undefined = $state()

  onMount(() => {
    if (!remote) void lsp.load().catch(() => {})
    doc.attach(host!)
    return () => {
      delete files.formatters[id]
      doc.detach()
    }
  })

  // An edit makes a preview tab a normal one.
  $effect(() => {
    if (doc.dirty) actions.keepTab(id)
  })

  // A line to show, asked for by a search result or a new tab; waits until the text is all in.
  $effect(() => {
    const j = files.jump
    if (j && j.tabId === id && doc.status === 'ready' && !doc.progress) {
      files.jump = null
      doc.goto(j.line)
    }
  })

  // Language server: the lazy chunk loads with the first file that has a ready server. Only this
  // tab, the one editing the file, tells the server about it, and not for files over 2 MiB.
  let canFormat = $state(false)
  let starting = $state(false)
  let symbols = $state<SymbolStep[]>([])
  let goSymbol: ((s: SymbolStep) => void) | undefined
  const server = $derived(remote ? null : lsp.serverFor(path))
  const serverId = $derived(doc.status === 'ready' && !doc.progress && doc.size <= MAX_LSP_BYTES && server?.state === 'ready' ? server.id : '')
  $effect(() => {
    const sid = serverId
    if (!sid) return
    let stop = false
    let off: (() => void) | null = null
    void import('./lib/lspbind').then((m) => {
      const s = lsp.server(sid)
      if (stop || !doc.view || !s) return
      const b = m.bindLsp({
        view: doc.view,
        projectId: app.currentId,
        path,
        server: s,
        writable: () => doc.writable,
        onSymbols: (steps) => (symbols = steps),
        // One command per tab: the button and the palette both run it, as a single edit.
        onFormat: (run, wait) => {
          canFormat = !!run
          starting = !!run && !!wait
          if (run) files.formatters[id] = () => (run(), doc.view?.focus())
          else delete files.formatters[id]
        },
      })
      off = () => b.dispose()
      goSymbol = b.go
      doc.unbind = off
    })
    return () => {
      stop = true
      off?.()
      canFormat = false
      starting = false
      symbols = []
      goSymbol = undefined
      delete files.formatters[id]
    }
  })

  // Preview draws markdown, CSV and HTML files. Source, the editor, is the default; a tab keeps its choice.
  const kind = untrack(() => (!remote ? previewKind(path) : null))
  let preview = $state(untrack(() => wasPreview(id)))
  let previewText = $state('')
  // A name ending in .pdf shows in the PDF frame, unless the user chose to see its text.
  let asText = $state(false)
  const pdf = $derived(!remote && !asText && isPdfPath(path) && (doc.status === 'binary' || doc.status === 'toolarge' || doc.status === 'ready'))
  const previewing = $derived(pdf || (!!kind && preview && doc.status === 'ready' && !doc.progress))
  $effect(() => {
    rememberPreview(id, preview)
  })
  // The text is read when Preview opens and again when the disk changes it; edits cannot happen meanwhile.
  $effect(() => {
    void doc.reloads
    if (previewing) previewText = doc.view?.state.doc.toString() ?? ''
  })
  function pick(on: boolean) {
    preview = on
    if (!on) void tick().then(() => doc.view?.focus())
  }

  let askNote = $state('')
  function ask() {
    const a = doc.selection()
    askNote = a ? attach.sendSelection(a) : 'Select some lines first.'
  }
</script>

<div class="file" data-editor-view>
  <div class="head">
    {#if remote}<span class="tag">{ssh.hostOfConn(remote)?.name ?? 'Remote'}</span>{/if}
    {#if remote}<span class="path" title={path}>{path}</span>{:else}<Crumbs {path} {symbols} onSymbol={(s) => goSymbol?.(s)} />{/if}
    {#if !doc.writable && doc.status === 'ready'}<span class="tag">Read-only</span>{/if}
    <span class="state" role="status">
      {#if doc.progress}Loading {formatBytes(doc.progress.loaded)} of {formatBytes(doc.progress.total)}…{:else if doc.saving}Saving…{:else if doc.dirty}Unsaved changes{:else if doc.updated}Updated from disk{/if}
    </span>
    {#if askNote}<span class="state" role="alert">{askNote}</span>{/if}
    {#if kind && doc.status === 'ready' && !doc.progress}
      <div class="switch" role="group" aria-label="View">
        <button type="button" aria-pressed={!preview} onclick={() => pick(false)}>Source</button>
        <button type="button" aria-pressed={preview} onclick={() => pick(true)}>Preview</button>
      </div>
    {/if}
    {#if doc.status === 'ready' && !remote && !previewing && !doc.progress}<FileBlame {doc} {id} {path} />{/if}
    {#if canFormat && !previewing}<button type="button" class="btn" disabled={!doc.writable || !!doc.progress || starting} title={starting ? 'The language server is still starting.' : undefined} onclick={() => files.formatters[id]?.()}>Format</button>{#if starting}<span class="state">The language server is starting…</span>{/if}{/if}
    {#if serverId && doc.view && !previewing}<FileRefactor {id} view={doc.view} writable={doc.writable} />{/if}
    {#if doc.status === 'ready' && !remote && !previewing}<button type="button" class="btn" disabled={!doc.selected} onclick={ask}>{doc.selected ? 'Ask the agent about these lines' : 'Select lines to ask the agent'}</button>{/if}
    {#if doc.status === 'ready' && (doc.dirty || doc.saving)}<button type="button" class="btn primary" disabled={doc.saving || !doc.writable || !!doc.progress} onclick={() => doc.save()}>Save</button>{/if}
  </div>

  <FileBars {doc} {id} {path} />
  {#if doc.notice}<p class="error" role="alert">{doc.notice}</p>{/if}

  {#if doc.status === 'loading'}
    <p class="note" role="status">Opening {baseName(path)}…</p>
  {:else if pdf}
    {#await import('./PdfView.svelte') then m}
      <m.default projectId={app.currentId} {path} size={doc.size} onText={doc.status === 'ready' ? () => (asText = true) : undefined} />
    {/await}
  {:else if (doc.status === 'binary' || doc.status === 'toolarge') && !remote && isImagePath(path)}
    {#await import('./ImageView.svelte') then m}
      <m.default projectId={app.currentId} {path} size={doc.size} />
    {/await}
  {:else if doc.status === 'binary'}
    <p class="note">This is a binary file, so it is not shown here. Open it with another program.</p>
  {:else if doc.status === 'toolarge'}
    <p class="note">This file is over 10 MB, which is more than the editor opens.</p>
  {:else if doc.status === 'error'}
    <p class="note error" role="alert">{doc.error}</p>
    {#if doc.retry}<div class="retry"><button type="button" class="btn" onclick={() => ((doc.status = 'loading'), doc.load())}>Try again</button></div>{/if}
  {/if}
  {#if previewing && kind}
    <div class="preview">
      {#await import('./Preview.svelte') then m}
        <m.default {kind} text={previewText} projectId={app.currentId} {path} />
      {/await}
    </div>
  {/if}
  <div class="stage" class:hidden={doc.status !== 'ready' || previewing}>
    <!-- The language server bar is a row above the code and pushes it down. -->
    {#if doc.status === 'ready' && server}<LspBar {server} {path} />{/if}
    <div class="host" class:hidden={doc.status !== 'ready'} bind:this={host} onfocusin={() => tabHint.on()} onfocusout={() => tabHint.off()}></div>
  </div>
  <!-- Below the code: it shows and hides as the editor gains and loses focus, and above the code that
       would move the bar's buttons between a mouse press and its release. -->
  {#if doc.status === 'ready' && !previewing}
    {#if tabHint.shown || !doc.colours}
      <p class="help" class:solo={doc.colours}>{#if tabHint.shown}<span class="keys">Tab indents. To leave the editor with the keyboard, press Esc, then Tab.</span>{/if}{#if !doc.colours} Colours are off for files over 2 MB.{/if}</p>
    {/if}
  {/if}
</div>

<style>
  .file {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .head {
    display: flex;
    flex: none;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-3);
    min-height: var(--bar-h);
    padding: var(--s-1) var(--s-3);
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .head :global(.btn) { min-height: var(--control-h-s); font-size: var(--fs-s); }
  .path {
    flex: 1 1 16ch;
    min-width: 0;
    overflow: hidden;
    font: var(--fs-s) var(--font-mono);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag,
  .state {
    flex: none;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .note,
  .help,
  .error {
    margin: 0;
    padding: var(--s-4);
  }
  .note {
    color: var(--muted);
  }
  .help {
    padding: var(--s-1) var(--s-3);
    border-top: 1px solid var(--border);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  @media (pointer: coarse) {
    .keys {
      display: none;
    }
    .help.solo {
      display: none;
    }
  }
  .retry {
    padding: 0 var(--s-4);
  }
  .stage {
    position: relative;
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .host {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
  .preview {
    display: flex;
    flex: 1;
    flex-direction: column;
    min-height: 0;
  }
  .switch {
    display: flex;
    flex: none;
    gap: var(--s-1);
  }
  .switch button {
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 1px solid transparent;
    border-radius: var(--r-s);
    background: none;
    color: var(--muted);
    font: inherit;
    font-size: var(--fs-s);
  }
  .switch button:hover {
    background: var(--hover);
  }
  .switch button[aria-pressed='true'] {
    background: var(--selected);
    color: var(--fg);
  }
  @media (max-width: 900px), (pointer: coarse) {
    .switch button {
      min-width: 44px;
      min-height: 44px;
    }
  }
  .hidden {
    display: none;
  }
</style>
