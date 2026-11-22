<script lang="ts">
  // The status board loads on first use, so the app shell stays small.
  let { onNavigate }: { onNavigate: () => void } = $props()
</script>

{#await import('./Board.svelte')}
  <p role="status">Loading the board…</p>
{:then board}
  <board.default {onNavigate} />
{:catch}
  <p role="alert">The board could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
