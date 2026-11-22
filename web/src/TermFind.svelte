<script lang="ts">
  import { onDestroy } from 'svelte'
  import { isMac, composing } from './lib/keys'
  import { terms } from './lib/termstore.svelte'

  let { id }: { id: string } = $props()

  let query = $state('')
  let found = $state<{ at: number; total: number } | null>(null)
  let field: HTMLInputElement | undefined = $state()

  const count = $derived(!query ? '' : !found || found.total === 0 ? 'No matches' : `${found.at} of ${found.total}`)

  function run(dir: 'up' | 'down' | 'same') {
    found = query ? (terms.views.get(id)?.find(query, dir) ?? null) : null
    if (!query) terms.views.get(id)?.clearFind()
  }

  function close() {
    terms.views.get(id)?.clearFind()
    terms.patch(id, { find: false })
    terms.views.get(id)?.focus()
  }

  function keys(e: KeyboardEvent) {
    if (composing(e)) return
    if (e.key === 'Escape') {
      e.preventDefault()
      close()
    } else if (e.key === 'Enter' && e.target === field) {
      e.preventDefault()
      run(e.shiftKey ? 'down' : 'up')
    }
  }

  // The selection belongs to the search: it goes when the bar does, even on a tab switch.
  onDestroy(() => terms.views.get(id)?.clearFind())

  // Keep the field's focus (and the phone keyboard) when a button is pressed.
  const keep = (e: Event) => e.preventDefault()
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions: Escape closes the bar from any control in it -->
<div class="bar" role="group" aria-label="Find in output" data-termfind={id} onkeydown={keys}>
  <input
    class="input"
    type="text"
    aria-label="Find in output"
    aria-keyshortcuts={isMac ? 'Meta+F' : 'Control+Shift+F'}
    placeholder="Find in output"
    autocomplete="off"
    autocapitalize="off"
    spellcheck="false"
    bind:this={field}
    bind:value={query}
    oninput={() => run('same')}
  />
  <span class="count" role="status" aria-live="polite">{count}</span>
  <button type="button" class="btn step" aria-label="Older match" title="Older match (Enter)" disabled={!query} onpointerdown={keep} onclick={() => run('up')}>↑</button>
  <button type="button" class="btn step" aria-label="Newer match" title="Newer match (Shift+Enter)" disabled={!query} onpointerdown={keep} onclick={() => run('down')}>↓</button>
  <button type="button" class="btn step" aria-label="Close find" title="Close (Esc)" onpointerdown={keep} onclick={close}>×</button>
</div>

<style>
  .bar {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2);
    background: var(--surface);
    border-top: 1px solid var(--border);
  }
  .bar input {
    flex: 1;
    min-width: 0;
  }
  .count {
    flex: none;
    min-width: 6ch;
    padding: 0 var(--s-1);
    color: var(--muted);
    font-size: var(--fs-s);
    text-align: right;
    white-space: nowrap;
  }
  .step {
    flex: none;
    min-width: var(--control-h);
    padding: 0;
    touch-action: manipulation;
  }
</style>
