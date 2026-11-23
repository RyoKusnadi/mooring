<script lang="ts">
  // One image at full size in a dialog. Esc or Close ends it.
  import { onMount } from 'svelte'

  let { src, name, size, onClose }: { src: string; name: string; size: string; onClose: () => void } = $props()
  let missing = $state(false)
  let dialog: HTMLDialogElement | undefined = $state()

  onMount(() => dialog?.showModal())
</script>

<dialog class="dlg wide" bind:this={dialog} aria-labelledby="viewer-h" onclose={onClose} onclick={(e) => e.target === dialog && dialog?.close()}>
  <header>
    <h2 id="viewer-h">{name} · {size}</h2>
    <button type="button" class="btn" onclick={() => dialog?.close()}>Close <kbd>Esc</kbd></button>
  </header>
  <div class="body">{#if missing}<p class="gone">Image not found</p>{:else}<img {src} alt={name} onerror={() => (missing = true)} />{/if}</div>
</dialog>

<style>
  header {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-3) var(--s-4);
    border-bottom: 1px solid var(--border);
  }
  h2 {
    flex: 1;
    min-width: 0;
    margin: 0;
    overflow: hidden;
    font-size: var(--fs-l);
    line-height: var(--lh-l);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  kbd {
    padding: 0 var(--s-1);
    border-radius: var(--r-s);
    background: var(--hover);
    color: var(--muted);
    font-size: var(--fs-s);
    line-height: var(--lh-s);
  }
  .body {
    display: grid;
    place-items: center;
    padding: var(--s-4);
    background: var(--bg);
  }
  .gone {
    margin: var(--s-6) 0;
    color: var(--muted);
  }
  img {
    display: block;
    width: auto;
    height: auto;
    max-width: 100%;
    max-height: calc(80dvh - 120px);
    border-radius: var(--r-m);
  }
</style>
