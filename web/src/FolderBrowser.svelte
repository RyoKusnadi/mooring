<script lang="ts">
  import { onMount } from 'svelte'
  import { app, type FolderList } from './lib/state.svelte'
  import { CommandError } from './lib/events'

  let {
    start,
    actionLabel,
    busy = false,
    error = '',
    onPick,
  }: { start: string; actionLabel: string; busy?: boolean; error?: string; onPick: (path: string) => void } = $props()

  let listing = $state<FolderList | null>(null)
  let typed = $state('')
  let loading = $state(true)
  let loadError = $state('')
  let ticket = 0

  let field: HTMLInputElement | undefined = $state()

  async function load(path: string, first = false) {
    const mine = ++ticket
    const typedBefore = typed
    loading = true
    loadError = ''
    try {
      const l = await app.run<FolderList>('fs.list', { path })
      if (mine !== ticket) return
      listing = l
      // Keep a path the user started typing while this folder was loading. Writing into a
      // field that has focus would put the home folder in front of what they are typing.
      if (typed === typedBefore && !(first && document.activeElement === field)) typed = l.path
    } catch (e) {
      if (mine !== ticket) return
      loadError = e instanceof CommandError ? e.message : 'Could not list that folder.'
    } finally {
      if (mine === ticket) loading = false
    }
  }

  const child = (name: string) => (listing!.path.endsWith('/') ? listing!.path + name : listing!.path + '/' + name)

  onMount(() => void load(start, true))
</script>

<div class="browser">
  <form
    class="path"
    onsubmit={(e) => {
      e.preventDefault()
      void load(typed.trim())
    }}
  >
    <div class="field">
      <label for="browse-path">Folder on this computer</label>
      <div class="row">
        <input id="browse-path" class="input mono" bind:this={field} bind:value={typed} autocomplete="off" spellcheck="false" aria-invalid={loadError ? 'true' : undefined} aria-describedby={loadError ? 'browse-error' : undefined} />
        <button type="submit" class="btn">Go</button>
      </div>
    </div>
  </form>
  {#if loadError}<p class="error" id="browse-error" role="alert">{loadError}</p>{/if}
  <div class="list" aria-busy={loading}>
    {#if listing}
      <button type="button" class="entry up" disabled={!listing.parent || listing.parent === listing.path} onclick={() => load(listing!.parent)}>
        <span aria-hidden="true">↑</span> Up one folder
      </button>
      {#each listing.entries.filter((e) => e.dir) as entry (entry.name)}
        <button type="button" class="entry" onclick={() => load(child(entry.name))}>
          <span class="n">{entry.name}</span>
          {#if entry.git}<span class="tag">git repository</span>{/if}
        </button>
      {:else}
        <p class="help pad">{loading ? 'Loading folders…' : 'No folders in here.'}</p>
      {/each}
    {:else}
      <p class="help pad" role="status">{loading ? 'Loading folders…' : 'Could not load folders.'}</p>
    {/if}
  </div>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <button type="button" class="btn primary" disabled={!listing || busy || loading || !!loadError} onclick={() => listing && onPick(listing.path)}>
    {busy ? 'Working…' : actionLabel}
  </button>
</div>

<style>
  .browser {
    display: grid;
    gap: var(--s-2);
    justify-items: start;
  }
  .browser > * {
    min-width: 0;
  }
  .path {
    width: 100%;
  }
  .row {
    display: flex;
    gap: var(--s-2);
  }
  .list {
    display: grid;
    align-content: start;
    width: 100%;
    height: 220px;
    padding: var(--s-1);
    overflow-y: auto;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    background: var(--raised);
  }
  .entry {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    min-height: var(--control-h);
    padding: 0 var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: inherit;
    text-align: left;
    transition: background-color var(--ease);
  }
  .entry:hover:not(:disabled) {
    background: var(--hover);
  }
  .entry:disabled {
    color: var(--muted);
  }
  .up {
    justify-content: start;
    color: var(--muted);
  }
  .n {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tag {
    flex: none;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .pad {
    padding: var(--s-2);
  }
</style>
