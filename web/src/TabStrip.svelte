<script lang="ts">
  // The tabs of one pane: kind icon, name, close on hover. Drag a tab to move it; arrows switch tabs.
  import Icon from './Icon.svelte'
  import TabMarks from './TabMarks.svelte'
  import TabMenu from './TabMenu.svelte'
  import { actions } from './lib/actions'
  import { insertIndex } from './lib/dropzone'
  import type { Group } from './lib/layout'
  import { tabIcon } from './lib/tabicon'

  let { group, label, anchor }: { group: Group; label: (t: { id: string }) => string; anchor: string } = $props()

  const MIME = 'application/x-mooring-tab'
  let insertAt = $state<number | null>(null)
  let strip: HTMLElement | undefined = $state()
  let fadeStart = $state(false)
  let fadeEnd = $state(false)

  let menu = $state<{ id: string; x: number; y: number } | null>(null)
  const menuTab = $derived(menu ? group.tabs.find((t) => t.id === menu!.id) : undefined)

  /** Right click, the menu key and Shift+F10 all arrive as contextmenu; a key press has no pointer, so the menu opens under the tab. */
  function onContext(e: MouseEvent | KeyboardEvent, id: string) {
    e.preventDefault()
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
    const key = e instanceof KeyboardEvent || (e.clientX === 0 && e.clientY === 0)
    menu = { id, x: key ? r.left : (e as MouseEvent).clientX, y: key ? r.bottom : (e as MouseEvent).clientY }
  }
  // Some systems send no contextmenu event for the menu key or Shift+F10, so both are handled here.
  const menuKey = (e: KeyboardEvent, id: string) => (e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')) && onContext(e, id)
  function closeMenu() {
    const id = menu?.id
    menu = null
    queueMicrotask(() => id && document.getElementById('tab-' + id)?.focus())
  }

  const isTabDrag = (e: DragEvent) => !!e.dataTransfer?.types.includes(MIME)

  function onDragStart(e: DragEvent, id: string) {
    e.dataTransfer?.setData(MIME, id)
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  }

  function barOver(e: DragEvent) {
    if (!isTabDrag(e) || !anchor) return
    e.preventDefault()
    const tabs = [...(e.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('[data-tab]')]
    insertAt = insertIndex(tabs.map((t) => t.getBoundingClientRect()), e.clientX)
  }

  function barDrop(e: DragEvent) {
    const id = e.dataTransfer?.getData(MIME)
    const at = insertAt
    insertAt = null
    if (!id || at === null) return
    e.preventDefault()
    actions.drop(id, anchor, 'center', at)
  }

  // A tab opened or picked elsewhere can sit past the end of a narrow strip (a phone): bring it in view.
  $effect(() => {
    const id = group.active
    // The whole tab, so its close button is not left under the "+" at the end of a narrow strip.
    if (id) document.getElementById('tab-' + id)?.closest('.tab')?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  })

  function measure() {
    if (!strip) return
    fadeStart = strip.scrollLeft > 1
    fadeEnd = strip.scrollLeft + strip.clientWidth < strip.scrollWidth - 1
  }

  $effect(() => {
    void group.tabs.length
    if (!strip) return
    measure()
    const watch = new ResizeObserver(measure)
    watch.observe(strip)
    return () => watch.disconnect()
  })

  function tabKeys(e: KeyboardEvent) {
    if (e.altKey || e.ctrlKey || e.metaKey) return
    if (e.key === 'Delete' && (e.target as HTMLElement).getAttribute('role') === 'tab') {
      e.preventDefault()
      return actions.closeTab(group.active)
    }
    const step = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const i = group.tabs.findIndex((t) => t.id === group.active)
    const next = group.tabs[(i + step + group.tabs.length) % group.tabs.length]
    actions.focusTab(next.id)
    queueMicrotask(() => document.getElementById('tab-' + next.id)?.focus())
  }
</script>

<div
  class="tabs"
  class:fade-start={fadeStart}
  class:fade-end={fadeEnd}
  role="tablist"
  aria-label="Tabs"
  tabindex="-1"
  bind:this={strip}
  onscroll={measure}
  onkeydown={tabKeys}
  ondragover={barOver}
  ondragleave={() => (insertAt = null)}
  ondrop={barDrop}
>
  {#each group.tabs as tab, i (tab.id)}
    <div class="tab" class:active={tab.id === group.active} class:before={insertAt === i} class:preview={tab.preview} class:pinned={tab.pinned} data-preview={tab.preview ? '' : undefined} data-tab data-kind={tab.kind} role="presentation" draggable="true" ondragstart={(e) => onDragStart(e, tab.id)} ondragend={() => (insertAt = null)}>
      <button
        type="button"
        role="tab"
        id={'tab-' + tab.id}
        aria-selected={tab.id === group.active}
        aria-controls={'panel-' + group.tabs[0].id}
        tabindex={tab.id === group.active ? 0 : -1}
        title={tab.preview ? 'Preview. Double-click or edit the file to keep it open.' : undefined}
        onclick={() => actions.focusTab(tab.id)}
        ondblclick={() => actions.keepTab(tab.id)}
        oncontextmenu={(e) => onContext(e, tab.id)}
        onkeydown={(e) => menuKey(e, tab.id)}
      ><TabMarks id={tab.id} host={tab.host} pinned={tab.pinned} /><Icon name={tabIcon(tab.kind)} /><span class="name">{label(tab)}</span></button>
      <button type="button" class="close" aria-label={`Close ${label(tab)}`} tabindex={tab.id === group.active ? 0 : -1} onclick={() => actions.closeTab(tab.id)}><Icon name="x" /></button>
    </div>
  {/each}
  {#if insertAt === group.tabs.length}<span class="end-mark" aria-hidden="true"></span>{/if}
</div>
{#if menu && menuTab}<TabMenu tab={menuTab} x={menu.x} y={menu.y} onClose={closeMenu} />{/if}

<style>
  .tabs {
    display: flex;
    flex: 1;
    align-items: stretch;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tabs.fade-end {
    mask-image: linear-gradient(to right, #000 calc(100% - 24px), transparent);
  }
  .tabs.fade-start {
    mask-image: linear-gradient(to left, #000 calc(100% - 24px), transparent);
  }
  .tabs.fade-start.fade-end {
    mask-image: linear-gradient(to right, transparent, #000 24px, #000 calc(100% - 24px), transparent);
  }
  .tab {
    position: relative;
    display: flex;
    flex: none;
    align-items: center;
    min-width: 96px;
    max-width: 180px;
    border-bottom: 2px solid transparent;
    transition: background-color var(--ease);
  }
  .tab:hover {
    background: var(--hover);
  }
  .tab.active {
    background: var(--bg);
    border-bottom-color: var(--accent);
  }
  .tab.before::before,
  .end-mark {
    content: '';
    position: absolute;
    left: 0;
    top: 4px;
    bottom: 4px;
    width: 2px;
    background: var(--accent);
  }
  .end-mark {
    position: static;
    flex: none;
    margin: 4px 0;
  }
  .tab button {
    min-height: 100%;
    border: 0;
    background: transparent;
    color: var(--fg);
    font: inherit;
    font-size: var(--fs-m);
  }
  .tab button[role='tab'] {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    padding: 0 0 0 var(--s-3);
    white-space: nowrap;
  }
  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .tab.pinned button[role='tab'] {
    padding-right: var(--s-3);
  }
  .tab.preview .name {
    font-style: italic;
  }
  .tab:not(.active) button[role='tab'] {
    color: var(--muted);
  }
  .tab .close {
    display: grid;
    flex: none;
    place-items: center;
    width: 24px;
    height: 24px;
    min-height: 0;
    margin-right: var(--s-1);
    border-radius: var(--r-s);
    color: var(--muted);
    opacity: 0;
  }
  .tab:hover .close,
  .tab.active .close,
  .close:focus-visible {
    opacity: 1;
  }
  .tab .close:hover {
    background: var(--border);
    color: var(--fg);
  }
  /* A touch screen: the close button is on the active tab only, so no hidden tap target sits on the others. */
  @media (pointer: coarse) {
    .tab .close {
      width: 44px;
      height: 44px;
      opacity: 1;
    }
    .tab:not(.active) .close {
      display: none;
    }
  }
  /* A pinned tab has no close button: Pin again to undo it, or use its menu. Last, to win over the rules above. */
  .tab.pinned .close,
  .tab.pinned:hover .close,
  .tab.pinned.active .close {
    display: none;
  }
</style>
