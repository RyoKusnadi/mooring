<script lang="ts">
  // The group view loads on first use, like the chat view.
  let { id }: { id: string } = $props()
</script>

{#await import('./ChatGroup.svelte')}
  <p role="status">Loading the group…</p>
{:then group}
  <group.default {id} />
{:catch}
  <p>The group view could not load. Reload the page and try again.</p>
{/await}

<style>
  p {
    margin: 0;
    padding: var(--s-5) var(--s-4);
    color: var(--muted);
  }
</style>
