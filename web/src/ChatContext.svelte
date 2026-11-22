<script lang="ts">
  // The summary a chat started from, and the prompt that asked for one, folded away by default.
  import { actions } from './lib/actions'
  import { chats } from './lib/chatstore.svelte'

  let { kind, text, fromChat, changes }: { kind: 'context' | 'summary'; text: string; fromChat?: string; changes?: string } = $props()
  const from = $derived(fromChat ? chats.list[fromChat] : undefined)
</script>

<details class="ctx">
  <summary>{kind === 'context' ? 'Summary this chat started from' : 'You asked for a summary of this chat'}</summary>
  <p class="text">{text}</p>
  {#if changes}
    <details class="changes">
      <summary>Uncommitted changes sent with the summary</summary>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <pre tabindex="0" role="region" aria-label="Uncommitted changes">{changes}</pre>
    </details>
  {/if}
  {#if fromChat}
    {#if from}
      <button type="button" class="btn ghost" onclick={() => actions.openChat(fromChat)}>Open {from.title ? `"${from.title}"` : 'the earlier chat'}</button>
    {:else}
      <p class="gone">The earlier chat is deleted.</p>
    {/if}
  {/if}
</details>

<style>
  summary {
    min-height: var(--control-h);
    color: var(--muted);
    font-size: var(--fs-s);
    line-height: var(--control-h);
  }
  .text {
    margin: 0 0 var(--s-2);
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    background: var(--surface);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  /* The ghost button's own padding would push its words right of the summary line above it. */
  .ghost {
    margin-inline-start: calc(-1 * (var(--s-3) + 1px));
  }
  .changes {
    margin-bottom: var(--s-2);
  }
  pre {
    max-height: 320px;
    margin: 0;
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    background: var(--surface);
    font-family: var(--font-mono);
    font-size: var(--fs-s);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    overflow: auto;
  }
  .gone {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
