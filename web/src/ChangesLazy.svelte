<script lang="ts">
  // The Changes view loads on first use, so the app shell stays small.
</script>

{#await import('./ChangesView.svelte')}
  <p role="status">Loading the changes…</p>
{:then view}
  <view.default />
{:catch}
  <p>The changes view could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
