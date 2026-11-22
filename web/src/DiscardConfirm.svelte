<script lang="ts">
  // The question before a discard: says what goes, where a copy lands, and defaults to keeping the changes.
  let { label, note, forever, busy, onrun, onkeep }: { label: string; note: string; forever: boolean; busy: boolean; onrun: () => void; onkeep: () => void } = $props()
  let keep: HTMLButtonElement | undefined = $state()
  let box: HTMLElement | undefined = $state()

  /** Shows the question (it can sit below a long list) and puts focus on Keep changes. */
  export function show() {
    keep?.focus()
    box?.scrollIntoView({ block: 'nearest' })
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions: Escape keeps the changes from anywhere in the question -->
<div class="confirm" role="group" aria-label="Confirm discard" bind:this={box} onkeydown={(e) => e.key === 'Escape' && (e.stopPropagation(), onkeep())}>
  <p>
    {#if forever}
      There is no trash available here. Discard {label} for good? {note} This cannot be undone.
    {:else}
      Discard {label}? {note} A copy goes to the trash first, so you can get it back from there.
    {/if}
  </p>
  <button type="button" class="btn danger small" disabled={busy} onclick={onrun}>{forever ? 'Discard for good' : 'Discard'}</button>
  <button type="button" class="btn small" bind:this={keep} onclick={onkeep}>Keep changes</button>
</div>

<style>
  .confirm {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1) var(--s-2);
    margin: var(--s-3) var(--s-2) var(--s-2);
    padding: var(--s-2);
    border: 1px solid var(--danger);
    border-radius: var(--r-s);
    background: var(--raised);
    font-size: var(--fs-s);
  }
  p {
    flex: 1 1 100%;
    margin: 0;
    overflow-wrap: anywhere;
  }
  .danger {
    color: var(--danger);
  }
  .small {
    min-height: calc(var(--control-h) - 4px);
    padding: 0 var(--s-1);
    font-size: var(--fs-s);
  }
  @media (pointer: coarse) {
    .small {
      min-height: 44px;
    }
  }
</style>
