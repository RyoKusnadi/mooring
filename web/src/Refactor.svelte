<script lang="ts">
  // Opens find references or rename for the editor that asked. Loads on first use (see Shell.svelte).
  import References from './References.svelte'
  import Rename from './Rename.svelte'
  import { refactorView } from './lib/refactorview.svelte'

  // Cancel and Close hand focus back to the editor; opening a file from the list does not.
  function close(focus = true) {
    const v = refactorView.request?.view
    refactorView.request = null
    if (focus) v?.focus()
  }
</script>

{#if refactorView.request}
  {#if refactorView.request.kind === 'refs'}<References request={refactorView.request} onClose={close} />{:else}<Rename request={refactorView.request} onClose={close} />{/if}
{/if}
