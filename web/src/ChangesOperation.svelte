<script lang="ts">
  // The banner for a merge, rebase, cherry-pick, revert or am that waits for the user: what is left, then Abort or Finish.
  import { operationText, type Operation } from './lib/gitoperation'
  import { git } from './lib/gitstore.svelte'

  let { op, left }: { op: Operation; left: number } = $props()
  const t = $derived(operationText(op, left))
  let asking = $state(false)
  let keep: HTMLButtonElement | undefined = $state()
  $effect(() => {
    if (asking) keep?.focus()
  })
  const busy = $derived(git.busy !== '')

  async function abort() {
    asking = false
    await git.finish('abort')
  }
</script>

<div class="op" role="group" aria-label={t.title}>
  <p role="status"><strong>{t.title}</strong> {t.body}</p>
  {#if asking}
    <p class="ask">{t.confirm}</p>
    <div class="row">
      <button type="button" class="btn small danger" disabled={busy} onclick={abort}>{t.abort}</button>
      <button type="button" class="btn small" bind:this={keep} onclick={() => (asking = false)}>Keep going</button>
    </div>
  {:else}
    <div class="row">
      <button type="button" class="btn small" disabled={busy} onclick={() => (asking = true)}>{t.abort}</button>
      <button type="button" class="btn small primary" class:off={left > 0} disabled={busy} aria-disabled={left > 0} aria-describedby={left > 0 ? 'op-why' : undefined} onclick={() => left === 0 && git.finish('continue')}>{t.finish}</button>
    </div>
    {#if left > 0}<span id="op-why" class="sr">{t.why}</span>{/if}
  {/if}
</div>

<style>
  .op {
    margin: var(--s-2);
    padding: var(--s-2);
    border: 1px solid var(--attention);
    border-radius: var(--r-l);
    background: var(--attention-bg);
    font-size: var(--fs-s);
  }
  p {
    margin: 0 0 var(--s-2);
  }
  .ask {
    color: var(--fg);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
  }
  .small {
    min-height: var(--control-h-s);
    padding: 0 var(--s-2);
    font-size: var(--fs-s);
  }
  .btn.off {
    border-color: transparent;
    background: var(--disabled-bg);
    color: var(--disabled-fg);
    cursor: not-allowed;
  }
  .danger {
    color: var(--danger);
  }
  @media (pointer: coarse) {
    .small {
      min-height: 44px;
    }
  }
</style>
