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
  import { baseName, formatBytes } from './lib/paths'
  import { isImagePath } from './lib/imagetype'
  import { ssh } from './lib/sshstore.svelte'
  import { tabHint } from './lib/tabhint.svelte'
  import { app } from './lib/state.svelte'

  /** `remote` is an SSH connection id: the file is on that host, and no language server runs for it. */
  let { id, path, remote = '' }: { id: string; path: string; remote?: string } = $props()

  // The view is keyed by tab and path, so these never change for one instance.
  const doc = untrack(() => new FileDoc(app.currentId, id, path, remote))
  let host: HTMLElement | undefined = $state()
  const closing = $derived(files.closing === id)

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

  let keepButton: HTMLButtonElement | undefined = $state()
  let reloadButton: HTMLButtonElement | undefined = $state()

  // A bar that appears moves focus to its safe button; dismissing it returns focus to the text.
  $effect(() => {
    if (closing) void tick().then(() => keepButton?.focus())
  })
  $effect(() => {
    if (doc.conflict && doc.conflict !== 'deleted' && doc.status === 'ready') void tick().then(() => reloadButton?.focus())
  })
  // Focus goes back to the editor at once and again when the action ends: a reload makes the
  // editor read-only while it loads, which drops focus to the page.
  async function dismiss(run: () => unknown) {
    const done = run()
    await tick()
    doc.view?.focus()
    await done
    await tick()
    doc.view?.focus()
  }

  async function saveAndClose() {
    await doc.save()
    if (!doc.dirty) actions.closeTab(id, true)
  }

  const CONFLICT = {
    changed: 'This file changed on disk, and you have unsaved edits here.',
    save: 'Not saved. The file on disk is newer than the version you opened.',
    deleted: 'This file was deleted from disk. Save to create it again.',
  } as const

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
    {#if canFormat}<button type="button" class="btn" disabled={!doc.writable || !!doc.progress || starting} title={starting ? 'The language server is still starting.' : undefined} onclick={() => files.formatters[id]?.()}>Format</button>{#if starting}<span class="state">The language server is starting…</span>{/if}{/if}
    {#if doc.status === 'ready' && !remote}<button type="button" class="btn" disabled={!doc.selected} onclick={ask}>{doc.selected ? 'Ask the agent about these lines' : 'Select lines to ask the agent'}</button>{/if}
    {#if doc.status === 'ready' && (doc.dirty || doc.saving)}<button type="button" class="btn primary" disabled={doc.saving || !doc.writable || !!doc.progress} onclick={() => doc.save()}>Save</button>{/if}
  </div>

  {#if closing}
    <div class="bar" role="alert">
      <span>Close {baseName(path)} without saving?</span>
      <button type="button" class="btn primary" onclick={saveAndClose}>Save and close</button>
      <button type="button" class="btn danger" onclick={() => actions.closeTab(id, true)}>Discard changes</button>
      <button type="button" class="btn" bind:this={keepButton} onclick={() => dismiss(() => (files.closing = ''))}>Keep editing</button>
    </div>
  {/if}
  {#if doc.conflict && doc.status === 'ready'}
    <div class="bar" role="alert">
      <span>{CONFLICT[doc.conflict]}</span>
      {#if doc.conflict !== 'deleted'}
        <button type="button" class="btn" bind:this={reloadButton} onclick={() => dismiss(() => doc.reloadTheirs())}>Load the file from disk</button>
        <button type="button" class="btn" onclick={() => dismiss(() => doc.keepMine())}>Overwrite the file on disk with my version</button>
      {/if}
    </div>
  {/if}
  {#if doc.notice}<p class="error" role="alert">{doc.notice}</p>{/if}

  {#if doc.status === 'loading'}
    <p class="note" role="status">Opening {baseName(path)}…</p>
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
  <div class="stage" class:hidden={doc.status !== 'ready'}>
    <!-- The language server bar is a row above the code and pushes it down. -->
    {#if doc.status === 'ready' && server}<LspBar {server} {path} />{/if}
    <div class="host" class:hidden={doc.status !== 'ready'} bind:this={host} onfocusin={() => tabHint.on()} onfocusout={() => tabHint.off()}></div>
  </div>
  <!-- Below the code: it shows and hides as the editor gains and loses focus, and above the code that
       would move the bar's buttons between a mouse press and its release. -->
  {#if doc.status === 'ready'}
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
  .bar {
    display: flex;
    flex: none;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
    background: var(--attention-bg);
    color: var(--attention);
    font-weight: 600;
  }
  .bar span {
    flex: 1 1 24ch;
    min-width: 0;
  }
  .bar .btn {
    color: var(--fg);
    background: var(--raised);
  }
  .bar .btn.primary {
    color: var(--on-accent);
    background: var(--accent);
  }
  .bar .btn.danger {
    color: var(--danger);
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
  .hidden {
    display: none;
  }
</style>
