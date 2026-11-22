<script lang="ts">
  // Under a diff: how many comments are waiting, and the buttons to send them to a chat or drop them.
  import { attach } from './lib/attachstore.svelte'
  import { diffComments } from './lib/diffcomments.svelte'
  import { app } from './lib/state.svelte'

  const project = $derived(app.currentId)
  const count = $derived(diffComments.list(project).length)
  let error = $state('')

  function send() {
    const { text, error: tooBig } = diffComments.prepare(project)
    error = tooBig
    if (tooBig) return
    const id = project
    error = attach.sendComments(text, () => diffComments.clear(id))
  }
</script>

{#if count > 0}
  <div class="bar" role="group" aria-label="Comments on diff lines">
    <span class="n">{count} {count === 1 ? 'comment' : 'comments'}</span>
    {#if error}<span class="error" role="alert">{error}</span>{/if}
    <button type="button" class="btn small primary" onclick={send}>Send to chat…</button>
    <button type="button" class="btn small" onclick={() => diffComments.clear(project)}>Discard comments</button>
  </div>
{/if}

<style>
  .bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-3);
    padding: var(--s-2) var(--s-4);
    border-top: 1px solid var(--border);
    background: var(--surface);
  }
  .n {
    flex: 1;
    min-width: 12ch;
    font-weight: 600;
  }
</style>
