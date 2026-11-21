<script lang="ts">
  import { terms } from './lib/termstore.svelte'

  let { id }: { id: string } = $props()
  const ctrl = $derived(terms.ui[id]?.ctrl ?? false)

  const keys: { label: string; name: string; data: string }[] = [
    { label: 'Esc', name: 'Escape', data: '\x1b' },
    { label: 'Tab', name: 'Tab', data: '\t' },
    { label: '←', name: 'Left arrow', data: '\x1b[D' },
    { label: '↓', name: 'Down arrow', data: '\x1b[B' },
    { label: '↑', name: 'Up arrow', data: '\x1b[A' },
    { label: '→', name: 'Right arrow', data: '\x1b[C' },
  ]

  // Keep focus in the terminal so the phone keyboard stays up.
  const keep = (e: Event) => e.preventDefault()
  const press = (data: string) => terms.views.get(id)?.press(data)
</script>

<div class="keys" role="toolbar" aria-label="Terminal keys">
  <button type="button" class="btn key" aria-pressed={ctrl} onpointerdown={keep} onclick={() => terms.patch(id, { ctrl: !ctrl })}>Ctrl</button>
  {#each keys as k (k.label)}
    <button type="button" class="btn key" aria-label={k.name} onpointerdown={keep} onclick={() => press(k.data)}>{k.label}</button>
  {/each}
</div>

<style>
  .keys {
    display: none;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2);
    background: var(--surface);
    border-top: 1px solid var(--border);
  }
  .keys button {
    flex: 1;
    padding: 0;
    background: var(--raised);
    touch-action: manipulation;
  }
  .keys button:active {
    background: var(--selected);
  }
  .keys button[aria-pressed='true'] {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--on-accent);
  }
  @media (max-width: 900px), (pointer: coarse) {
    .keys {
      display: flex;
    }
  }
</style>
