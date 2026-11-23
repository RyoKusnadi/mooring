<script lang="ts">
  // The short commit box of the sidebar. It shares its message with the Changes view; Push and Amend stay there.
  import { composing } from './lib/keys'
  import { stagedOf } from './lib/gitapi'
  import { git } from './lib/gitstore.svelte'

  const staged = $derived(git.status ? stagedOf(git.status).length : 0)
  const can = $derived(git.busy === '' && staged > 0 && git.message.trim() !== '' && !git.amend)
  // Why Commit is off, shown once there is something to act on: the other half is missing.
  const reason = $derived(git.busy !== '' || git.amend || can ? '' : staged === 0 && git.message.trim() === '' ? '' : staged === 0 ? 'Stage a file to commit' : 'Write a message to commit')
  const failure = $derived(git.failure && !git.failure.source ? git.failure.message : '')
  const key = /Mac|iPhone|iPad/.test(navigator.platform) ? 'Cmd+Enter' : 'Ctrl+Enter'
  let field: HTMLTextAreaElement | undefined = $state()

  // One line that grows with the text, up to about five lines.
  $effect(() => {
    void git.message
    if (!field) return
    field.style.height = 'auto'
    field.style.height = Math.min(field.scrollHeight, 110) + 'px'
  })

  function keys(e: KeyboardEvent) {
    if (e.key === 'Enter' && !composing(e) && (e.metaKey || e.ctrlKey) && can) {
      e.preventDefault()
      void git.commit()
    }
  }
</script>

<div class="box">
  <textarea bind:this={field} class="input" rows="1" aria-label="Commit message" placeholder="Commit message" title={`Commit with ${key}`} aria-describedby={reason ? 'commit-why' : undefined} spellcheck="true" bind:value={git.message} onkeydown={keys}></textarea>
  <button type="button" class="btn primary" disabled={!can} onclick={() => git.commit()}>Commit</button>
</div>
{#if reason}<p class="note" id="commit-why">{reason}</p>{/if}
{#if git.amend}<p class="note">Amend is on. Commit from Changes.</p>{/if}
{#if failure}<p class="note err" role="alert">{failure}</p>{:else if git.notice}<p class="note" role="status">{git.notice}</p>{/if}

<style>
  .box {
    display: flex;
    align-items: flex-start;
    gap: var(--s-1);
  }
  textarea {
    flex: 1;
    min-width: 0;
    min-height: var(--control-h);
    max-height: 110px;
    /* One line sits in the middle of the control, like the inputs around it. The line height is set so the maths holds. */
    line-height: var(--lh-m);
    padding-block: calc((var(--control-h) - 2px - var(--lh-m)) / 2);
    resize: none;
  }
  .note {
    margin: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
    white-space: pre-line;
  }
  .err {
    color: var(--danger);
  }
</style>
