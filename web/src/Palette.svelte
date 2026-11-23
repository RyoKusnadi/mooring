<script lang="ts">
  import { composing } from './lib/keys'
  import { app } from './lib/state.svelte'
  import { onMount } from 'svelte'
  import { filterItems, looksLikeFile, type PaletteItem } from './lib/filter'
  import { files } from './lib/filestore.svelte'

  let { items, onClose }: { items: PaletteItem[]; onClose: () => void } = $props()

  let dialog: HTMLDialogElement | undefined = $state()
  let query = $state('')
  let active = $state(0)

  // A file name typed here offers the file finder first.
  const shown = $derived.by(() => {
    const found = filterItems(items, query)
    if (!app.layout || !looksLikeFile(query)) return found
    const q = query.trim()
    return [{ id: 'file-named', label: `List files named ${q}`, run: () => ((files.finderQuery = q), (files.finder = 'file')) }, ...found]
  })
  const current = $derived(Math.min(active, Math.max(shown.length - 1, 0)))

  onMount(() => dialog?.showModal())

  function pick(item: PaletteItem | undefined) {
    if (!item) return
    dialog?.close()
    item.run()
  }

  function keys(e: KeyboardEvent) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const n = shown.length
      if (n) active = (current + (e.key === 'ArrowDown' ? 1 : -1) + n) % n
    } else if (e.key === 'Enter' && !composing(e)) {
      e.preventDefault()
      pick(shown[current])
    }
  }

  $effect(() => {
    document.getElementById('palette-' + shown[current]?.id)?.scrollIntoView({ block: 'nearest' })
  })
</script>

<dialog bind:this={dialog} class="dlg" aria-label="Command palette" onclose={onClose} onclick={(e) => e.target === dialog && dialog.close()}>
  <input
    class="q"
    type="text"
    role="combobox"
    aria-expanded="true"
    aria-controls="palette-list"
    aria-activedescendant={shown[current] ? 'palette-' + shown[current].id : undefined}
    aria-label="Type a command"
    placeholder="Type a command, a project or a file name"
    autocomplete="off"
    spellcheck="false"
    bind:value={query}
    oninput={() => (active = 0)}
    onkeydown={keys}
  />
  <ul id="palette-list" role="listbox" aria-label="Commands">
    {#each shown as item, i (item.id)}
      <!-- Keyboard selection happens in the input above; the rows only need the pointer. -->
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <li id={'palette-' + item.id} role="option" aria-selected={i === current} onclick={() => pick(item)} onpointermove={() => (active = i)}>
        <span>{item.label}</span>
        {#if item.hint && !app.narrow}<kbd>{item.hint}</kbd>{/if}
      </li>
    {:else}
      <li class="none" role="presentation">Nothing matches “{query}”.</li>
    {/each}
  </ul>
  {#if !app.narrow}<p class="keyhelp">Shortcuts with ⌥ (Option) run even in text boxes, so on a Mac ⌥O, ⌥A and similar do not type special characters. On Windows and Linux, Ctrl+P, Ctrl+Shift+F and Ctrl+, open Mooring's tools instead of the browser's.</p>{/if}
</dialog>

<style>
  .q {
    width: 100%;
    min-height: calc(var(--control-h) + var(--s-2));
    padding: 0 var(--s-3);
    border: 0;
    border-bottom: 1px solid var(--border);
    border-radius: var(--r-m) var(--r-m) 0 0;
    background: transparent;
    color: var(--fg);
    font: inherit;
  }
  .q:focus-visible {
    outline-offset: -2px;
  }
  ul {
    max-height: min(340px, 50dvh);
    margin: 0;
    padding: var(--s-1);
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    min-height: var(--row-h);
    padding: 0 var(--s-2);
    border-radius: var(--r-s);
  }
  li[aria-selected='true'] {
    background: var(--selected);
  }
  .none {
    color: var(--muted);
  }
  kbd {
    flex: none;
    color: var(--muted);
    font: var(--fs-s) var(--font-ui);
  }
  .keyhelp {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    border-top: 1px solid var(--border);
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
