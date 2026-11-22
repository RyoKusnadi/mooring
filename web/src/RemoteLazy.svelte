<script lang="ts">
  // The remote folder browser loads on first use, so the app shell stays small.
  let { conn, host }: { conn: string; host: string } = $props()
</script>

{#await import('./RemoteFiles.svelte')}
  <p role="status">Loading the file browser…</p>
{:then view}
  <view.default {conn} {host} />
{:catch}
  <p>The file browser could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
