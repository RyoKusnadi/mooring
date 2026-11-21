<script lang="ts">
  // The frame every SSH dialog shares: a native modal dialog with a title and a close button.
  // The element marked data-first gets the focus when it opens.
  import { onMount, type Snippet } from 'svelte'

  // onCancel runs on the Escape key (a keydown, because Chrome stops allowing cancel events to be prevented after two); returning true says it handled the key (a step went back), so the dialog stays open.
  // Two widths (480, or 640 with `wide`) and one top edge (10dvh). `footer` holds the main action, on the right.
  let { title, onClose, children, footer, wide = false, onCancel }: { title: string; onClose: () => void; children: Snippet; footer?: Snippet; wide?: boolean; top?: boolean; onCancel?: () => boolean } = $props()

  let dialog: HTMLDialogElement | undefined = $state()
  const uid = $props.id()

  onMount(() => {
    dialog?.showModal()
    dialog?.querySelector<HTMLElement>('[data-first]')?.focus()
  })
</script>

<dialog bind:this={dialog} class="dlg" class:wide aria-labelledby="{uid}-title" onclose={onClose} onkeydown={(e) => e.key === 'Escape' && onCancel?.() && e.preventDefault()}>
  <header>
    <h2 id="{uid}-title">{title}</h2>
    <button type="button" class="btn ghost close" aria-label="Close dialog" onclick={() => dialog?.close()}>×</button>
  </header>
  <div class="body">{@render children()}</div>
  {#if footer}<footer>{@render footer()}</footer>{/if}
</dialog>

<style>
  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--s-2);
    padding: var(--s-3) var(--s-3) 0 var(--s-4);
  }
  h2 {
    margin: var(--s-1) 0 0;
    min-width: 0;
    font-size: var(--fs-l);
    line-height: 1.3;
    overflow-wrap: anywhere;
  }
  .close {
    flex: none;
    min-width: var(--control-h);
    padding: 0;
  }
  .body {
    padding: var(--s-2) var(--s-4) var(--s-4);
  }
</style>
