<script lang="ts">
  // The settings dialog. Loads on first use (see Shell.svelte).
  import Settings from './Settings.svelte'
  import { settings } from './lib/settingsstore.svelte'

  let { onSignOut }: { onSignOut: () => void } = $props()
</script>

<div data-settings-dialog hidden></div>
{#if settings.dialog}<Settings onClose={() => (settings.dialog = false)} {onSignOut} />{/if}
