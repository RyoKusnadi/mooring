<script lang="ts">
  // The drag edge of the sidebar. Arrow keys move it by 16px (Shift: 64px); Home resets it; double click resets it.
  import { SIDEBAR_MAX, SIDEBAR_MIN, clampWidth } from './lib/sidebarwidth'

  let { width, onChange, onCommit, onReset }: { width: number; onChange: (w: number) => void; onCommit: (w: number) => void; onReset: () => void } = $props()

  let dragging = $state(false)
  let startX = 0
  let startW = 0

  function down(e: PointerEvent) {
    dragging = true
    startX = e.clientX
    startW = width
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  function move(e: PointerEvent) {
    if (dragging) onChange(clampWidth(startW + e.clientX - startX))
  }

  function up(e: PointerEvent) {
    if (!dragging) return
    dragging = false
    onCommit(clampWidth(startW + e.clientX - startX))
  }

  /** A cancelled drag (the system took the pointer) puts the width back and saves nothing. */
  function cancel() {
    if (!dragging) return
    dragging = false
    onChange(startW)
  }

  function keys(e: KeyboardEvent) {
    const step = e.shiftKey ? 64 : 16
    let next = width
    if (e.key === 'ArrowLeft') next = width - step
    else if (e.key === 'ArrowRight') next = width + step
    else if (e.key === 'Home') return void (e.preventDefault(), onReset())
    else return
    e.preventDefault()
    next = clampWidth(next)
    onChange(next)
    onCommit(next)
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions: a focusable separator is the window-splitter pattern -->
<div
  class="handle"
  class:dragging
  role="separator"
  aria-orientation="vertical"
  aria-label="Sidebar width"
  aria-valuemin={SIDEBAR_MIN}
  aria-valuemax={SIDEBAR_MAX}
  aria-valuenow={width}
  tabindex="0"
  onpointerdown={down}
  onpointermove={move}
  onpointerup={up}
  onpointercancel={cancel}
  ondblclick={onReset}
  onkeydown={keys}
></div>

<style>
  .handle {
    position: absolute;
    z-index: 3;
    top: 0;
    right: -3px;
    bottom: 0;
    width: 7px;
    cursor: col-resize;
    touch-action: none;
  }
  .handle:hover::after,
  .handle.dragging::after {
    content: '';
    position: absolute;
    inset: 0 3px;
    background: var(--accent);
  }
  .handle:focus-visible {
    outline-offset: -2px;
  }
</style>
