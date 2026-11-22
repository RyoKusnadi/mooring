<script lang="ts">
  // The Problems dialog. Loads on first use (see Shell.svelte).
  import Problems from './Problems.svelte'
  import { problemsView } from './lib/problemsview.svelte'
</script>

{#if problemsView.dialog}<Problems onClose={() => (problemsView.dialog = false)} />{/if}
