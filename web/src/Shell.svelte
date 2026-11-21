<script lang="ts">
  import { onMount, tick, untrack } from 'svelte'
  import AddProject from './AddProject.svelte'
  import FindDialog from './FindDialog.svelte'
  import ChatPickDialog from './ChatPickDialog.svelte'
  import { attach } from './lib/attachstore.svelte'
  import LspDialog from './LspDialog.svelte'
  import Palette from './Palette.svelte'
  import Sidebar from './Sidebar.svelte'
  import StatusBar from './StatusBar.svelte'
  import Workspace from './Workspace.svelte'
  import { actions, handleShortcut, paletteKey } from './lib/actions'
  import { needCount, showAttention } from './lib/attention'
  import { chats } from './lib/chatstore.svelte'
  import { terms } from './lib/termstore.svelte'
  import { chordOf, composing } from './lib/keys'
  import { paletteItems } from './lib/paletteitems'
  import SidebarHandle from './SidebarHandle.svelte'
  import { SIDEBAR_DEFAULT, browserStore, fitWidth, readWidth, writeWidth } from './lib/sidebarwidth'
  import { checkSession, logout } from './lib/api'
  import type { PaletteItem } from './lib/filter'
  import { files } from './lib/filestore.svelte'
  import { git } from './lib/gitstore.svelte'
  import { ssh } from './lib/sshstore.svelte'
  import { app } from './lib/state.svelte'
  import { accounts } from './lib/accountstore.svelte'
  import { tools } from './lib/toolstore.svelte'
  import { settings } from './lib/settingsstore.svelte'

  let { onSignOut }: { onSignOut: () => void } = $props()

  let adding = $state(false)
  let palette = $state(false)
  let drawer = $state(false)
  let sidebarW = $state(readWidth(browserStore()))
  let winW = $state(window.innerWidth)
  const shownW = $derived(fitWidth(sidebarW, winW))

  onMount(() => {
    app.start()
    void ssh.load().catch(() => {})
    void tools.load().catch(() => {})
    void accounts.load().catch(() => {})
    void settings.load().catch(() => {})
    return () => app.stop()
  })

  // The browser asks before the page goes away while any file tab, shown or not, has unsaved edits.
  $effect(() => {
    if (!files.anyDirty) return
    const stop = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', stop)
    return () => window.removeEventListener('beforeunload', stop)
  })

  // Chats that wait on the user show in the page title and on the favicon, so a hidden tab says so.
  $effect(() => showAttention(needCount(chats.attentionCount(), terms.attentionCount())))

  // Show changes (Alt+G, the palette, a chat's button) opens the view; on a phone the drawer gets out of its way.
  $effect(() => {
    if (git.reveal > 0 && untrack(() => app.narrow)) void setDrawer(false)
  })

  // Reveal in files (a tab's menu, a breadcrumb) shows the tree; on a phone that is the drawer.
  $effect(() => {
    if (files.revealTick > 0 && untrack(() => app.narrow)) void setDrawer(true)
  })

  // A socket refused for a dead session just looks like a dropped connection, so ask over HTTP.
  $effect(() => {
    if (app.status === 'live') return
    const timer = setInterval(() => {
      checkSession().then(
        (s) => s === 'signed-out' && leave(),
        () => {},
      )
    }, 5000)
    return () => clearInterval(timer)
  })

  function leave() {
    app.stop()
    onSignOut()
  }

  async function signOut() {
    try {
      await logout()
      leave()
    } catch {
      app.notice = "Could not sign out. Check that Mooring's server is running."
    }
  }

  const items = $derived<PaletteItem[]>(paletteItems({ addProject: () => (adding = true), languageServers: () => (settings.lsp = true), signOut }))

  function keydown(e: KeyboardEvent) {
    // An open dialog (Move to, a confirm) takes Escape first; the drawer closes on the next one.
    if (e.key === 'Escape' && drawer && !adding && !palette && !document.querySelector('dialog[open]')) {
      e.preventDefault()
      void setDrawer(false)
      return
    }
    if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.code === 'KeyK') {
      e.preventDefault()
      if (!adding) palette = !palette
      return
    }
    if (composing(e)) return
    if (adding || settings.lsp || palette || (files.finder && files.finder !== 'text') || ssh.modal || tools.dialog || accounts.dialog || settings.dialog || e.defaultPrevented) return
    const t = e.target as HTMLElement | null
    const inTerminal = !!t?.closest('[data-terminal]')
    const chord = chordOf(e, inTerminal)
    if (chord) {
      e.preventDefault()
      if (chord === 'palette') palette = true
      else if (chord === 'settings') actions.openSettings()
      else if (app.layout) openFinder(chord === 'file' ? 'file' : 'text')
      return
    }
    // Alt+letter works in a text box too; Alt+arrows and Alt+Esc stay with the box (they move by word).
    const typing = !!t && /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) && !inTerminal
    if (handleShortcut(e, typing)) e.preventDefault()
  }

  // The results panel is not modal, so its chords stay live: Alt+F refocuses it; Alt+O swaps it for the file finder.
  function openFinder(mode: 'file' | 'text') {
    if (files.finder === 'text' && mode === 'text') document.getElementById('find-q')?.focus()
    else files.finder = mode
  }

  let projectsButton: HTMLButtonElement | undefined = $state()
  // The dialogs open from the drawer, which closes behind them; on a phone focus goes back to Projects.
  let hadDialog = false
  $effect(() => {
    const open = tools.dialog || ssh.dialog || accounts.dialog || settings.dialog
    if (hadDialog && !open && untrack(() => app.narrow)) void tick().then(() => document.activeElement === document.body && projectsButton?.focus())
    hadDialog = open
  })

  async function setDrawer(open: boolean) {
    drawer = open
    await tick()
    if (open) document.querySelector<HTMLElement>('#sidebar button')?.focus()
    else projectsButton?.focus()
  }
</script>

<svelte:window onkeydown={keydown} bind:innerWidth={winW} />

<div class="shell" class:narrow={app.narrow} style:--sidebar-w="{shownW}px">
  {#if app.narrow}
    <header inert={drawer}>
      <button type="button" class="btn ghost" aria-expanded={drawer} aria-controls="sidebar" bind:this={projectsButton} onclick={() => setDrawer(!drawer)}>Projects</button>
      <button type="button" class="btn ghost" aria-label={`Commands (${paletteKey})`} onclick={() => (palette = true)}>Commands</button>
    </header>
  {/if}
  <aside id="sidebar" class:open={drawer} inert={app.narrow && !drawer}>
    <Sidebar
      onAdd={() => {
        drawer = false
        adding = true
      }}
      onPalette={() => {
        drawer = false
        palette = true
      }}
      onNavigate={() => (drawer = false)}
    />
    {#if !app.narrow}<SidebarHandle width={shownW} onChange={(w) => (sidebarW = fitWidth(w, winW))} onCommit={(w) => writeWidth(browserStore(), fitWidth(w, winW))} onReset={() => ((sidebarW = SIDEBAR_DEFAULT), writeWidth(browserStore(), SIDEBAR_DEFAULT))} />{/if}
  </aside>
  {#if app.narrow && drawer}
    <button type="button" class="scrim" aria-label="Close projects" tabindex="-1" onclick={() => setDrawer(false)}></button>
  {/if}
  <main inert={app.narrow && drawer}>
    <Workspace onAdd={() => (adding = true)} />
  </main>
  <StatusBar />
</div>

{#if adding}<AddProject onClose={() => (adding = false)} />{/if}
{#if files.finder}{#key files.finder}<FindDialog />{/key}{/if}
{#if attach.asking}<ChatPickDialog />{/if}
{#if settings.lsp}<LspDialog onClose={() => (settings.lsp = false)} />{/if}
{#if ssh.modal}{#await import('./SshOverlays.svelte') then m}<m.default />{/await}{/if}
{#if tools.dialog}{#await import('./ToolOverlays.svelte') then m}<m.default />{/await}{/if}
{#if accounts.dialog}{#await import('./AccountOverlays.svelte') then m}<m.default />{/await}{/if}
{#if settings.dialog}{#await import('./SettingsOverlays.svelte') then m}<m.default onSignOut={signOut} />{/await}{/if}
{#if palette}<Palette {items} onClose={() => (palette = false)} />{/if}

<style>
  .shell {
    display: grid;
    grid-template-columns: var(--sidebar-w, 264px) minmax(0, 1fr);
    grid-template-rows: minmax(0, 1fr) auto;
    height: 100dvh;
  }
  aside {
    position: relative;
    grid-row: 1;
    grid-column: 1;
    min-height: 0;
  }
  main {
    grid-row: 1;
    grid-column: 2;
    min-width: 0;
    min-height: 0;
    padding-right: env(safe-area-inset-right, 0px);
  }
  .shell > :global(footer) {
    grid-column: 1 / -1;
  }
  .narrow {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr) auto;
  }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    min-height: var(--bar-h);
    padding: env(safe-area-inset-top, 0px) max(var(--s-2), env(safe-area-inset-right, 0px)) 0 max(var(--s-2), env(safe-area-inset-left, 0px));
    background: var(--surface);
    border-bottom: 1px solid var(--border);
  }
  .narrow aside {
    position: fixed;
    z-index: 20;
    inset: 0 auto 0 0;
    width: min(300px, 86vw);
    transform: translateX(-100%);
    transition: transform var(--ease);
    visibility: hidden;
  }
  .narrow aside.open {
    transform: none;
    visibility: visible;
  }
  .narrow main {
    grid-row: 2;
    grid-column: 1;
  }
  .scrim {
    position: fixed;
    z-index: 10;
    inset: 0;
    border: 0;
    background: var(--scrim);
  }
  .narrow :global(footer) {
    grid-row: 3;
  }
</style>
