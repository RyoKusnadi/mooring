<script lang="ts">
  // The tool servers dialog. Loads on first use (see Shell.svelte).
  import ToolServers from './ToolServers.svelte'
  import { tools } from './lib/toolstore.svelte'
</script>

<div data-tool-servers hidden></div>
{#if tools.dialog}<ToolServers onClose={() => (tools.dialog = false)} />{/if}
