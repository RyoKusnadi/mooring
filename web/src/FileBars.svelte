<script lang="ts">
  // The two bars above a file: the one that asks before closing a file with unsaved edits, and the
  // one shown when the disk and the editor disagree.
  import { tick } from 'svelte'
  import { actions } from './lib/actions'
  import type { FileDoc } from './lib/filedoc.svelte'
  import { files } from './lib/filestore.svelte'
  import { baseName } from './lib/paths'

  let { doc, id, path }: { doc: FileDoc; id: string; path: string } = $props()
  const closing = $derived(files.closing === id)

  let keepButton: HTMLButtonElement | undefined = $state()
  let reloadButton: HTMLButtonElement | undefined = $state()

  // A bar that appears moves focus to its safe button; dismissing it returns focus to the text.
  $effect(() => {
    if (closing) void tick().then(() => keepButton?.focus())
  })
  $effect(() => {
    if (doc.conflict && doc.conflict !== 'deleted' && doc.status === 'ready') void tick().then(() => reloadButton?.focus())
  })
  // Focus goes back to the editor at once and again when the action ends: a reload makes the
  // editor read-only while it loads, which drops focus to the page.
  async function dismiss(run: () => unknown) {
    const done = run()
    await tick()
    doc.view?.focus()
    await done
    await tick()
    doc.view?.focus()
  }

  async function saveAndClose() {
    await doc.save()
    if (!doc.dirty) actions.closeTab(id, true)
  }

  const CONFLICT = {
    changed: 'This file changed on disk, and you have unsaved edits here.',
    save: 'Not saved. The file on disk is newer than the version you opened.',
    deleted: 'This file was deleted from disk. Save to create it again.',
  } as const
</script>

{#if closing}
  <div class="bar" role="alert">
    <span>Close {baseName(path)} without saving?</span>
    <button type="button" class="btn primary" onclick={saveAndClose}>Save and close</button>
    <button type="button" class="btn danger" onclick={() => actions.closeTab(id, true)}>Discard changes</button>
    <button type="button" class="btn" bind:this={keepButton} onclick={() => dismiss(() => (files.closing = ''))}>Keep editing</button>
  </div>
{/if}
{#if doc.conflict && doc.status === 'ready'}
  <div class="bar" role="alert">
    <span>{CONFLICT[doc.conflict]}</span>
    {#if doc.conflict !== 'deleted'}
      <button type="button" class="btn" bind:this={reloadButton} onclick={() => dismiss(() => doc.reloadTheirs())}>Load the file from disk</button>
      <button type="button" class="btn" onclick={() => dismiss(() => doc.keepMine())}>Overwrite the file on disk with my version</button>
    {/if}
  </div>
{/if}

<style>
  .bar {
    display: flex;
    flex: none;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
    background: var(--attention-bg);
    color: var(--attention);
    font-weight: 600;
  }
  .bar span {
    flex: 1 1 24ch;
    min-width: 0;
  }
  .bar .btn {
    color: var(--fg);
    background: var(--raised);
  }
  .bar .btn.primary {
    color: var(--on-accent);
    background: var(--accent);
  }
  .bar .btn.danger {
    color: var(--danger);
  }
</style>
