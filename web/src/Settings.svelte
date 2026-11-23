<script lang="ts">
  // One Settings dialog with a section list. Sections that already have a dialog (accounts, tool
  // servers, SSH hosts, language servers) open it from a button; token saving and appearance sit inside.
  import SettingsAppearance from './SettingsAppearance.svelte'
  import SettingsSignIn from './SettingsSignIn.svelte'
  import SettingsNotify from './SettingsNotify.svelte'
  import SettingsSaving from './SettingsSaving.svelte'
  import SshModal from './SshModal.svelte'
  import { actions } from './lib/actions'
  import { settings } from './lib/settingsstore.svelte'

  let { onClose, onSignOut }: { onClose: () => void; onSignOut: () => void } = $props()

  interface Section {
    id: string
    name: string
    about?: string
    open?: () => void
  }
  const SECTIONS: Section[] = [
    { id: 'accounts', name: 'Accounts', about: 'Sign in to an agent provider, add a second login, and see each login\'s limits and usage.', open: actions.openAccounts },
    { id: 'tools', name: 'Tool servers', about: 'MCP servers your agents can use: add one from the catalog, check it, and choose who may use it.', open: actions.openTools },
    { id: 'ssh', name: 'SSH hosts', about: 'Saved hosts for remote terminals and files. Keys and passwords stay in the secret store.', open: actions.openHosts },
    { id: 'lsp', name: 'Language servers', about: 'Programs that give the editor errors, hover text and go to definition, one per language.', open: () => (settings.lsp = true) },
    { id: 'signin', name: 'Sign-in' },
    { id: 'saving', name: 'Token saving' },
    { id: 'notify', name: 'Notifications' },
    { id: 'appearance', name: 'Appearance' },
  ]

  let current = $state('accounts')
  const section = $derived(SECTIONS.find((s) => s.id === current) ?? SECTIONS[0])

  /** Closes Settings, then opens the section's own dialog. */
  function launch(open: () => void) {
    onClose()
    open()
  }
</script>

<SshModal title="Settings" {onClose} wide>
  <div class="frame">
    <nav aria-label="Settings sections">
      {#each SECTIONS as s, i (s.id)}
        <button type="button" class="item" data-first={i === 0 ? '' : undefined} aria-current={s.id === current ? 'true' : undefined} onclick={() => (current = s.id)}>{s.name}</button>
      {/each}
    </nav>
    <section aria-labelledby="settings-section-title">
      <h3 id="settings-section-title">{section.name}</h3>
      {#if section.open}
        <p class="help">{section.about}</p>
        <button type="button" class="btn" onclick={() => launch(section.open!)}>Open {section.name}</button>
      {:else if section.id === 'signin'}
        <SettingsSignIn />
      {:else if section.id === 'saving'}
        <SettingsSaving />
      {:else if section.id === 'notify'}
        <SettingsNotify />
      {:else}
        <SettingsAppearance />
      {/if}
    </section>
  </div>
  {#snippet footer()}
    <button type="button" class="btn out" onclick={() => (onClose(), onSignOut())}>Sign out</button>
    <button type="button" class="btn primary" onclick={onClose}>Done</button>
  {/snippet}
</SshModal>

<style>
  .frame {
    display: grid;
    grid-template-columns: 160px minmax(0, 1fr);
    gap: var(--s-4);
    min-height: 320px;
  }
  nav {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .item {
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: none;
    color: var(--muted);
    font: inherit;
    text-align: left;
  }
  .item:hover {
    background: var(--hover);
    color: var(--fg);
  }
  .item[aria-current='true'] {
    background: var(--selected);
    color: var(--fg);
  }
  h3 {
    margin: 0 0 var(--s-3);
    font-size: var(--fs-l);
  }
  .help {
    margin: 0 0 var(--s-3);
  }
  .out {
    margin-right: auto;
  }
  @media (max-width: 900px) {
    .frame {
      grid-template-columns: minmax(0, 1fr);
      gap: var(--s-3);
      min-height: 0;
    }
    nav {
      flex-direction: row;
      padding-bottom: var(--s-2);
      overflow-x: auto;
      border-bottom: 1px solid var(--border);
    }
    .item {
      flex: none;
    }
  }
</style>
