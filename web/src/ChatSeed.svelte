<script module lang="ts">
  export const MAX_CONTEXT = 16 * 1024
</script>

<script lang="ts">
  // A summary of an earlier chat, on the start screen of the new one. The user edits it; what is
  // left goes to the agent with the first message.
  import { chats } from './lib/chatstore.svelte'
  import { actions } from './lib/actions'

  let { id }: { id: string } = $props()

  const seed = $derived(chats.seeds[id])
  const from = $derived(seed ? chats.list[seed.fromChat] : undefined)
  const bytes = $derived(seed ? new TextEncoder().encode(seed.text).length : 0)
  const uid = $props.id()
</script>

{#if seed}
  <section class="seed" aria-labelledby="{uid}-h">
    <h3 id="{uid}-h">Summary of {from?.title ? `"${from.title}"` : 'the earlier chat'}</h3>
    <p class="help" id="{uid}-help">
      {seed.source === 'agent' ? 'The agent wrote this.' : 'The agent could not answer, so Mooring cut this from the messages.'}
      Edit it, then pick an agent. The agent reads it before your first message. Clear the box to start without it.
    </p>
    <textarea class="input" rows="10" aria-labelledby="{uid}-h" aria-describedby="{uid}-help" bind:value={seed.text}></textarea>
    {#if bytes > MAX_CONTEXT}
      <p class="error" role="alert">The summary is {Math.ceil(bytes / 1024)} KB. The limit is 16 KB. Cut some of it.</p>
    {/if}
    <button type="button" class="btn ghost" onclick={() => actions.openChat(seed.fromChat)}>Open the earlier chat</button>
  </section>
{/if}

<style>
  .seed {
    display: grid;
    gap: var(--s-2);
    justify-items: start;
    width: 100%;
    max-width: 64ch;
  }
  h3 {
    margin: 0;
    font-size: var(--fs-m);
  }
  p {
    margin: 0;
  }
  textarea {
    width: 100%;
    resize: vertical;
    font: inherit;
  }
</style>
