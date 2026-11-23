<script lang="ts">
  // The delete question under a tree row: move to the trash, or (when there is no trash) delete for good.
  import { tick } from 'svelte'
  import { actions } from './lib/actions'
  import { plainError, type Entry } from './lib/filesapi'
  import { files } from './lib/filestore.svelte'

  let { entry, path, isDir, depth, op, onError }: { entry: Entry; path: string; isDir: boolean; depth: number; op: 'delete' | 'delete-forever'; onError: (m: string) => void } = $props()

  let paying = $state(false)
  let confirmButton: HTMLButtonElement | undefined = $state()
  let keepButton: HTMLButtonElement | undefined = $state()
  // A move to the trash can be undone, so it starts on the action; deleting for good starts on Keep.
  $effect(() => {
    if (op === 'delete') void tick().then(() => confirmButton?.focus())
    else void tick().then(() => keepButton?.focus())
  })

  async function remove(permanent: boolean) {
    paying = true
    try {
      await actions.deleteEntry(path, permanent)
      files.editing = null
    } catch (e) {
      const code = (e as { code?: string }).code
      if (code === 'TRASH_UNAVAILABLE') files.editing = { op: 'delete-forever', path }
      else {
        files.editing = null
        onError('Could not delete ' + entry.name + '. ' + plainError(e))
      }
    } finally {
      paying = false
    }
  }

  // Esc keeps the item, like the Keep button.
  function keepOnEscape(e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    e.stopPropagation()
    files.editing = null
  }
</script>

<div class="ask" style:--depth={depth} role="group" aria-label={`Delete ${entry.name}`}>
  <p>
    {#if op === 'delete'}Move {entry.name}{isDir ? ' and what is in it' : ''} to the trash?{:else}There is no trash to move it to. Delete {entry.name}{isDir ? ' and what is in it' : ''} for good? This cannot be undone.{/if}
  </p>
  <div class="buttons">
    <button type="button" bind:this={confirmButton} class="btn danger solid small" disabled={paying} onkeydown={keepOnEscape} onclick={() => remove(op === 'delete-forever')}>{op === 'delete' ? 'Move to trash' : 'Delete permanently'}</button>
    <button type="button" bind:this={keepButton} class="btn small" onkeydown={keepOnEscape} onclick={() => (files.editing = null)}>Keep</button>
  </div>
</div>

<style>
  .ask {
    display: grid;
    gap: var(--s-2);
    margin: var(--s-1) 0;
    padding: var(--s-2) var(--s-2) var(--s-2) calc(var(--s-2) + var(--depth, 0) * var(--s-4));
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--raised);
  }
  p {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .small {
    padding: 0 var(--s-2);
  }
</style>
