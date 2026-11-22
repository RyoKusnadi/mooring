<script lang="ts">
  // The editor loads on first use, like the terminal and the chat, so the app shell stays small.
  let { id, path, remote = '' }: { id: string; path: string; remote?: string } = $props()
</script>

{#await import('./FileView.svelte')}
  <p role="status">Loading the editor…</p>
{:then view}
  <view.default {id} {path} {remote} />
{:catch}
  <p>The editor could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
