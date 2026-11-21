<script lang="ts">
  // The history view and the commit tab load on first use, so the app shell stays small.
  let { folder, commit = '' }: { folder: string; commit?: string } = $props()
</script>

{#await import('./HistoryView.svelte')}
  <p role="status">Loading the history…</p>
{:then view}
  <view.default {folder} {commit} />
{:catch}
  <p>The history view could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
