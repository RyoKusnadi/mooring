<script lang="ts">
  import BoardLazy from './BoardLazy.svelte'
  import ChangesSection from './ChangesSection.svelte'
  import ChatsHandle from './ChatsHandle.svelte'
  import ChatsSection from './ChatsSection.svelte'
  import FilesSection from './FilesSection.svelte'
  import Icon from './Icon.svelte'
  import ProjectRow from './ProjectRow.svelte'
  import SidebarTabs from './SidebarTabs.svelte'
  import { actions, paletteKey } from './lib/actions'
  import { appearance } from './lib/appearance.svelte'
  import { boardCounts, onBoard } from './lib/board'
  import { chats } from './lib/chatstore.svelte'
  import { git } from './lib/gitstore.svelte'
  import { split } from './lib/sidebarsplit.svelte'
  import { app } from './lib/state.svelte'

  let { onAdd, onPalette, onNavigate }: { onAdd: () => void; onPalette: () => void; onNavigate: () => void } = $props()

  let error = $state('')
  // The sizes the Chats edge works from: the sidebar, and the Chats block as it stands.
  let navH = $state(0)
  let chatsH = $state(0)
  const counts = $derived(boardCounts(onBoard(Object.values(chats.list), app.projects.map((p) => p.id))))
  const badge = $derived(counts.waiting + counts.finished)
  const themeLabel = $derived(appearance.drawn === 'light' ? 'Switch to dark theme' : 'Switch to light theme')

  /** Opens a dialog from the footer; on a phone the drawer closes first. */
  const open = (run: () => void) => () => {
    onNavigate()
    run()
  }
</script>

<nav aria-label="Projects" bind:clientHeight={navH}>
  <div class="head">
    <span class="name">Mooring</span>
  </div>
  <div class="heading">
    <h2>Projects</h2>
    <button type="button" class="icon-btn" aria-label="Add project" title="Add project" onclick={onAdd}><Icon name="plus" /></button>
  </div>
  {#if !app.ready}
    <p class="hint">Loading projects…</p>
  {:else if app.projects.length > 0}
    <ul style:--rows={Math.min(3, app.projects.length)}>
      {#each app.projects as project, i (project.id)}
        <ProjectRow
          {project}
          index={i}
          onPick={() => {
            error = ''
            app.select(project.id)
            onNavigate()
          }}
          onError={(m) => (error = m)}
        />
      {/each}
    </ul>
  {/if}
  {#if app.ready && app.current}
    <SidebarTabs {counts} {badge} />
  {/if}
  <!-- Both stay mounted: the file tree keeps its watch and the changes keep reading while the other shows. -->
  <div class="pane" hidden={git.view !== 'files'}><FilesSection {onNavigate} {onAdd} /></div>
  <div class="pane" hidden={git.view !== 'changes'}><ChangesSection {onNavigate} /></div>
  <!-- The board replaces the Chats list while it shows: it lists the chats of every project. -->
  {#if git.view === 'board'}
    <div class="pane"><BoardLazy {onNavigate} /></div>
  {:else}
    {#if split.open}<ChatsHandle height={split.height ?? chatsH} max={Math.max(0, navH - 280)} />{/if}
    <div class="chats" class:closed={!split.open} class:fixed={split.open && split.height !== null} style:--chats-h={split.open && split.height !== null ? `${split.height}px` : undefined} bind:clientHeight={chatsH}><ChatsSection {onNavigate} /></div>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  <div class="foot">
    <button type="button" class="row" onclick={onPalette}><Icon name="search" /><span class="grow">Search commands</span><kbd>{paletteKey}</kbd></button>
    <button type="button" class="icon-btn" aria-label={themeLabel} title={themeLabel} onclick={() => appearance.toggleTheme()}><Icon name={appearance.drawn === 'light' ? 'moon' : 'sun'} /></button>
    <button type="button" class="icon-btn" aria-label="Settings" title="Settings" onclick={open(() => actions.openSettings())}><Icon name="settings" /></button>
  </div>
</nav>

<style>
  nav {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow-y: auto;
    padding-bottom: env(safe-area-inset-bottom, 0px);
    padding-left: env(safe-area-inset-left, 0px);
    background: var(--surface);
    border-right: 1px solid var(--border);
    container: side / inline-size;
  }
  .head {
    display: flex;
    align-items: center;
    min-height: var(--bar-h);
    gap: var(--s-1);
    padding: 0 var(--s-2) 0 var(--s-3);
  }
  .name {
    flex: 1;
    font-weight: var(--fw-semibold);
  }
  .heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--s-2) var(--s-2) 0 var(--s-3);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
    color: var(--muted);
  }
  /* Never shrinks (it would vanish when the sidebar is full); capped, and scrolls past the cap.
     It always shows at least three rows, or all of them when there are fewer. */
  ul {
    flex: 0 0 auto;
    max-height: 35%;
    min-height: calc(var(--row-h) * var(--rows, 1));
    margin: 0;
    padding: 0 var(--s-2);
    overflow-y: auto;
  }
  .pane {
    display: flex;
    flex: 1 1 0;
    flex-direction: column;
    min-height: 120px;
  }
  /* Never shrinks below its list: when the sidebar is full it scrolls, instead of the chats sliding under
     the buttons at the bottom. Until the edge above it is dragged, it is as tall as its list, up to 40%. */
  .chats {
    display: flex;
    flex: 0 0 auto;
    flex-direction: column;
    max-height: 40%;
    min-height: 0;
  }
  .chats.fixed {
    height: var(--chats-h);
    max-height: calc(100% - 280px);
  }
  .chats.closed {
    border-top: 1px solid var(--border);
  }
  .pane[hidden] {
    display: none;
  }
  .hint,
  .error {
    margin: 0;
    padding: var(--s-1) var(--s-3);
  }
  .hint {
    flex: 1;
    color: var(--muted);
  }
  .error {
    font-size: var(--fs-s);
  }
  /* Sticky so Sign out and the dialogs stay on screen when the lists above are long. */
  .foot {
    position: sticky;
    bottom: 0;
    display: flex;
    align-items: center;
    gap: 0;
    margin-top: auto;
    padding: var(--s-1);
    border-top: 1px solid var(--border);
    background: var(--surface);
  }
  .row {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    min-height: var(--row-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: none;
    color: var(--muted);
    font: inherit;
    text-align: left;
  }
  .grow {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  kbd {
    flex: none;
    color: var(--faint);
    font: var(--fs-s) var(--font-ui);
  }
  .row:hover {
    background: var(--hover);
    color: var(--fg);
  }
  /* The shortcut hint goes when the row is too narrow to hold it beside the label, and a touch screen has no use for it. */
  @container side (max-width: 300px) {
    kbd {
      display: none;
    }
  }
  @container side (max-width: 250px) {
    .row {
      gap: var(--s-1);
      padding: 0 var(--s-1);
    }
  }
  @media (pointer: coarse) {
    kbd {
      display: none;
    }
  }
</style>
