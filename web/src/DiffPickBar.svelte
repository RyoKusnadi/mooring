<script lang="ts">
  // The bar over a diff when lines can be chosen: how many are chosen, and the buttons that act on them.
  let {
    count,
    staged,
    busy,
    note,
    onapply,
    onclear,
  }: { count: number; staged: boolean; busy: boolean; note: string; onapply: () => void; onclear: () => void } = $props()
  let tip = $state(false)
  let wrap: HTMLElement | undefined = $state()
  const outside = (e: MouseEvent) => {
    if (tip && !(e.target as Element).closest('.q, .tip')) tip = false
  }
  const escape = (e: KeyboardEvent) => {
    if (tip && e.key === 'Escape') {
      tip = false
      e.stopPropagation()
      wrap?.parentElement?.querySelector<HTMLElement>('.q')?.focus()
    }
  }
</script>

<svelte:window onclick={outside} onkeydown={escape} />

<!-- Sits inside the diff header, so the first code line starts right under one 40px row. -->
<span class="bar" bind:this={wrap}>
  <button type="button" class="btn primary" disabled={busy || count === 0} onclick={onapply}>
    {staged ? 'Unstage lines' : 'Stage lines'}{count > 0 ? ` (${count})` : ''}
  </button>
  <button type="button" class="btn" disabled={count === 0} onclick={onclear}>Clear choice</button>
  <span class="help" class:note={!!note} id="pick-help" role="status">
    {#if note}{note}{:else if count > 0}{count} {count === 1 ? 'line' : 'lines'} chosen.{/if}
  </span>
  {#if !note && count === 0}
    <button type="button" class="btn ghost q" aria-expanded={tip} aria-controls="pick-tip" aria-label="How to choose lines" onclick={() => (tip = !tip)}>?</button>
  {/if}
  {#if tip && !note && count === 0}
    <p class="tip" id="pick-tip">
      <span class="fine">Choose lines by clicking, shift-clicking or dragging. With the keyboard, arrows move, Space chooses a line, B chooses its block.</span>
      <span class="coarse">Tap lines to choose them.</span>
    </p>
  {/if}
</span>

<style>
  .coarse {
    display: none;
  }
  @media (pointer: coarse) {
    .fine {
      display: none;
    }
    .coarse {
      display: inline;
    }
  }
  .bar {
    display: contents;
  }
  .help {
    align-self: center;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .help:empty {
    display: none;
  }
  .help.note {
    flex: 1 1 100%;
    overflow-wrap: anywhere;
  }
  .q {
    width: var(--control-h);
    padding: 0;
    color: var(--muted);
  }
  .tip {
    flex: 1 1 100%;
    margin: 0;
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    background: var(--raised);
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
