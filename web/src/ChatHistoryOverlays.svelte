<script lang="ts">
  // The chat history dialog. Loads on first use (see Shell.svelte).
  import ChatHistory from './ChatHistory.svelte'
  import { chatHistory } from './lib/chathistorystore.svelte'
</script>

{#if chatHistory.dialog}<ChatHistory onClose={() => (chatHistory.dialog = false)} />{/if}
