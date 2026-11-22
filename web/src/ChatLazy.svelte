<script lang="ts">
  // The chat view loads on first use, like the terminal, so the app shell stays small.
  let { id }: { id: string } = $props()
</script>

{#await import('./Chat.svelte')}
  <p role="status">Loading the chat…</p>
{:then chat}
  <chat.default {id} />
{:catch}
  <p>The chat view could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
