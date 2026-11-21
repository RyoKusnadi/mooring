<script lang="ts">
  import Self from './Node.svelte'
  import Pane from './Pane.svelte'
  import { isSplit, type Layout, type Node } from './lib/layout'
  import { resizeSplit } from './lib/layout-edit'
  import { app } from './lib/state.svelte'

  let { node, path = [] }: { node: Node; path?: number[] } = $props()

  let el: HTMLElement | undefined = $state()
  let drag: { start: Layout; origin: number; span: number; index: number } | null = null

  function down(e: PointerEvent, index: number) {
    if (!app.layout || !el || !isSplit(node)) return
    const r = el.getBoundingClientRect()
    const row = node.split === 'row'
    drag = { start: app.layout, origin: row ? e.clientX : e.clientY, span: row ? r.width : r.height, index }
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  function move(e: PointerEvent) {
    if (!drag || !isSplit(node)) return
    const d = drag
    const now = node.split === 'row' ? e.clientX : e.clientY
    app.edit(() => resizeSplit(d.start, path, d.index, (now - d.origin) / d.span))
  }

  function key(e: KeyboardEvent, index: number) {
    if (!isSplit(node)) return
    const row = node.split === 'row'
    const back = row ? 'ArrowLeft' : 'ArrowUp'
    const fwd = row ? 'ArrowRight' : 'ArrowDown'
    if (e.key !== back && e.key !== fwd) return
    e.preventDefault()
    const step = (e.shiftKey ? 0.1 : 0.02) * (e.key === back ? -1 : 1)
    app.edit((l) => resizeSplit(l, path, index, step))
  }
</script>

{#if isSplit(node)}
  <div class="split {node.split}" bind:this={el}>
    {#each node.children as child, i (i)}
      <div class="cell" style:flex="{node.sizes[i]} 1 0">
        <Self node={child} path={[...path, i]} />
      </div>
      {#if i < node.children.length - 1}
        <!-- A separator you can focus is the ARIA window splitter pattern. -->
        <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
        <div
          class="divider"
          role="separator"
          tabindex="0"
          aria-label="Resize panes"
          aria-orientation={node.split === 'row' ? 'vertical' : 'horizontal'}
          aria-valuemin="10"
          aria-valuemax="90"
          aria-valuenow={Math.round(node.sizes[i] * 100)}
          onpointerdown={(e) => down(e, i)}
          onpointermove={move}
          onpointerup={() => (drag = null)}
          onpointercancel={() => (drag = null)}
          onkeydown={(e) => key(e, i)}
        ></div>
      {/if}
    {/each}
  </div>
{:else}
  <Pane group={node} />
{/if}

<style>
  .split {
    display: flex;
    width: 100%;
    height: 100%;
    min-width: 0;
    min-height: 0;
  }
  .split.column {
    flex-direction: column;
  }
  .cell {
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }
  .divider {
    position: relative;
    flex: none;
    background: var(--border);
    transition: background-color var(--ease);
  }
  .row > .divider {
    width: 1px;
    cursor: col-resize;
  }
  .column > .divider {
    height: 1px;
    cursor: row-resize;
  }
  .divider::before {
    content: '';
    position: absolute;
    inset: -4px;
  }
  .divider:hover,
  .divider:active {
    background: var(--accent);
  }
  .divider {
    touch-action: none;
  }
</style>
