<script lang="ts">
  import ChangesSection from './ChangesSection.svelte'
  import ChatsSection from './ChatsSection.svelte'
  import FilesSection from './FilesSection.svelte'
  import Icon from './Icon.svelte'
  import ProjectRow from './ProjectRow.svelte'
  import { actions, paletteKey } from './lib/actions'
  import { appearance } from './lib/appearance.svelte'
  import { git } from './lib/gitstore.svelte'
  import { app } from './lib/state.svelte'

  let { onAdd, onPalette, onNavigate }: { onAdd: () => void; onPalette: () => void; onNavigate: () => void } = $props()

  let error = $state('')
  const themeLabel = $derived(appearance.drawn === 'light' ? 'Switch to dark theme' : 'Switch to light theme')

  /** Opens a dialog from the footer; on a phone the drawer closes first. */
  const open = (run: () => void) => () => {
    onNavigate()
    run()
  }
</script>

<nav aria-label="Projects">
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
    <div class="switch" role="group" aria-label="Show">
      <button type="button" aria-pressed={git.view === 'files'} onclick={() => (git.view = 'files')}>Files</button>
      <button type="button" aria-pressed={git.view === 'changes'} aria-label={git.status?.repo && git.status.files.length > 0 ? `Changes, ${git.status.files.length} files` : undefined} onclick={() => (git.view = 'changes')}>
        Changes{#if git.status?.repo && git.status.files.length > 0}<span class="n">{git.status.files.length}</span>{/if}
      </button>
    </div>
  {/if}
  <!-- Both stay mounted: the file tree keeps its watch and the changes keep reading while the other shows. -->
  <div class="pane" hidden={git.view !== 'files'}><FilesSection {onNavigate} {onAdd} /></div>
  <div class="pane" hidden={git.view !== 'changes'}><ChangesSection {onNavigate} /></div>
  <div class="chats"><ChatsSection {onNavigate} /></div>
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
    font-weight: 600;
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
    font-weight: 600;
    color: var(--muted);
  }
  /* Never shrinks (it would vanish when the sidebar is full); capped, and scrolls past the cap.
     It always shows at least three rows, or all of them when there are fewer. */
  ul {
    flex: 0 0 auto;
    max-height: 35%;
    min-height: calc(var(--control-h) * var(--rows, 1));
    margin: 0;
    padding: 0 var(--s-2);
    overflow-y: auto;
  }
  .switch {
    display: flex;
    gap: var(--s-1);
    padding: var(--s-2) var(--s-2) 0;
  }
  .switch button {
    display: inline-flex;
    flex: 1;
    align-items: center;
    justify-content: center;
    gap: var(--s-1);
    min-height: var(--control-h);
    border: 1px solid transparent;
    border-radius: var(--r-s);
    background: none;
    color: var(--muted);
    font: inherit;
    font-weight: 500;
  }
  .switch button:hover {
    background: var(--hover);
  }
  .switch button[aria-pressed='true'] {
    background: var(--selected);
    color: var(--fg);
  }
  .n {
    font-weight: 400;
  }
  .pane {
    display: flex;
    flex: 1 1 0;
    flex-direction: column;
    min-height: 120px;
  }
  /* Never shrinks below its list (capped at 40%): when the sidebar is full it scrolls,
     instead of the chats sliding under the buttons at the bottom. */
  .chats {
    display: flex;
    flex: 0 0 auto;
    flex-direction: column;
    max-height: 40%;
    min-height: 0;
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
    gap: var(--s-1);
    margin-top: auto;
    padding: var(--s-1) var(--s-2);
    border-top: 1px solid var(--border);
    background: var(--surface);
  }
  .row {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    min-height: 28px;
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
  }
  kbd {
    color: var(--faint);
    font: var(--fs-s) var(--font-ui);
  }
  .row:hover {
    background: var(--hover);
    color: var(--fg);
  }
  @media (pointer: coarse), (max-width: 900px) {
    .row {
      min-height: 44px;
    }
  }
</style>
