<script lang="ts">
  // The edge between Files and Chats. Drag it, or use Up and Down while it has focus (16px; Shift 64px). Home resets it.
  import { CHATS_MAX, CHATS_MIN, CHATS_STEP, clampHeight, split } from './lib/sidebarsplit.svelte'

  /** The height the Chats block has now, and the most the sidebar leaves it. */
  let { height, max }: { height: number; max: number } = $props()
  const top = $derived(Math.max(CHATS_MIN, Math.min(CHATS_MAX, max)))

  let dragging = $state(false)
  let startY = 0
  let startH = 0

  function down(e: PointerEvent) {
    dragging = true
    startY = e.clientY
    startH = height
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  // Dragging up makes Chats taller.
  const at = (e: PointerEvent) => Math.min(top, clampHeight(startH + startY - e.clientY))

  function move(e: PointerEvent) {
    if (dragging) split.setHeight(at(e), false)
  }

  function up(e: PointerEvent) {
    if (!dragging) return
    dragging = false
    split.setHeight(at(e))
  }

  function cancel() {
    if (!dragging) return
    dragging = false
    split.setHeight(startH, false)
  }

  function keys(e: KeyboardEvent) {
    const step = e.shiftKey ? CHATS_STEP * 4 : CHATS_STEP
    if (e.key === 'Home') return void (e.preventDefault(), split.setHeight(null))
    if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
    e.preventDefault()
    split.setHeight(Math.min(top, height + (e.key === 'ArrowUp' ? step : -step)))
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions: a focusable separator is the window-splitter pattern -->
<div
  class="handle"
  class:dragging
  role="separator"
  aria-orientation="horizontal"
  aria-label="Chats height"
  title="Drag to resize. Home resets."
  aria-valuemin={CHATS_MIN}
  aria-valuemax={Math.max(CHATS_MIN, top)}
  aria-valuenow={Math.round(height)}
  tabindex="0"
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={cancel}
  ondblclick={() => split.setHeight(null)}
  onkeydown={keys}
></div>

<style>
  /* The grab area sits between Files and Chats and takes its own height, so it never covers a row.
     The line is drawn in the middle; a finger gets 44px and a short grip to show it can be dragged. */
  .handle {
    position: relative;
    flex: none;
    height: 8px;
    cursor: row-resize;
    touch-action: none;
  }
  .handle::before {
    content: '';
    position: absolute;
    inset: 50% 0 auto;
    height: 1px;
    background: var(--border);
  }
  .handle:hover::before,
  .handle.dragging::before,
  .handle:focus-visible::before {
    background: var(--accent);
  }
  .handle:focus-visible {
    outline-offset: -2px;
  }
  @media (pointer: coarse) {
    .handle {
      height: var(--control-h);
    }
    .handle::after {
      content: '';
      position: absolute;
      inset: calc(50% - 2px) calc(50% - 16px) auto;
      height: 4px;
      border-radius: var(--r-s);
      background: var(--border-strong);
    }
  }
</style>
