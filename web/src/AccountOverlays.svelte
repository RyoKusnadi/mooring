<script lang="ts">
  // The accounts dialog. Loads on first use (see Shell.svelte).
  import Accounts from './Accounts.svelte'
  import { accounts } from './lib/accountstore.svelte'
</script>

<div data-accounts-dialog hidden></div>
{#if accounts.dialog}<Accounts onClose={() => (accounts.dialog = false)} />{/if}
