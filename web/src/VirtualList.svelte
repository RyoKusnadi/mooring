<script lang="ts">
  import { onMount, untrack, type Snippet } from 'svelte'
  import { buildOffsets, canScroll, followAfterScroll, visibleRange } from './lib/virtual'

  let {
    count,
    row,
    label,
    estimate = 72,
    follow = $bindable(true),
    jump = 'Jump to latest',
    stick = true,
    onrange,
  }: {
    count: number
    row: Snippet<[number]>
    label: string
    estimate?: number
    follow?: boolean
    /** The text of the jump button; an empty text shows no button. */
    jump?: string
    /** False for lists that grow at the end (history pages): new rows never move the scroll position. */
    stick?: boolean
    /** Called with the first and last row in view, so a list can fetch more near its end. */
    onrange?: (first: number, last: number) => void
  } = $props()

  // Only the rows in view (plus a few) are in the page. Heights are measured once a row
  // has been drawn; until then a row counts as `estimate`.
  const OVER = 4
  const NEAR_BOTTOM = 40

  let viewport: HTMLElement | undefined = $state()
  let top = $state(0)
  let height = $state(0)
  let version = $state(0)
  const heights: number[] = []
  let observer: ResizeObserver | undefined
  let frame = 0
  let shift = 0
  // What moved the scroll position: the user (wheel, touch, key, held scrollbar) or this list.
  let userAt = -Infinity
  let held = false
  let programmatic = false
  let lastTop = 0
  const touched = () => (userAt = performance.now())
  const byUser = () => held || performance.now() - userAt < 600
  const place = (y: number) => {
    if (!viewport || viewport.scrollTop === y) return
    programmatic = true
    viewport.scrollTop = y
    // The scroll event comes before the next frame; a move that changed nothing must not leave the flag set.
    requestAnimationFrame(() => (programmatic = false))
  }

  const offsets = $derived.by(() => {
    void version
    return buildOffsets(count, heights, estimate)
  })
  const range = $derived(visibleRange(offsets, top, height, OVER))
  const total = $derived(offsets[count])
  /** The button only helps when there is more below than fits in view. */
  const scrollable = $derived(canScroll(total, height, NEAR_BOTTOM))
  const rows = $derived(Array.from({ length: range[1] - range[0] }, (_, k) => range[0] + k))

  function measured(entries: ResizeObserverEntry[]) {
    for (const e of entries) {
      const i = Number((e.target as HTMLElement).dataset.i)
      const h = e.borderBoxSize[0]?.blockSize ?? e.contentRect.height
      const before = heights[i] ?? estimate
      if (heights[i] === h) continue
      // A row above the viewport changed: move the scroll position so what is read stays put.
      if (!follow && offsets[i] + before <= top) shift += h - before
      heights[i] = h
    }
    if (!frame) frame = requestAnimationFrame(flush)
  }

  function flush() {
    frame = 0
    version++
    if (shift && viewport) place(viewport.scrollTop + shift)
    shift = 0
  }

  function measure(node: HTMLElement) {
    observer?.observe(node)
    return { destroy: () => observer?.unobserve(node) }
  }

  onMount(() => {
    observer = new ResizeObserver(measured)
    for (const el of viewport?.querySelectorAll('[data-i]') ?? []) observer.observe(el)
    return () => {
      observer?.disconnect()
      cancelAnimationFrame(frame)
    }
  })

  function onScroll() {
    if (!viewport) return
    top = viewport.scrollTop
    follow = followAfterScroll({
      follow,
      user: byUser(),
      programmatic,
      prevTop: lastTop,
      top,
      distance: viewport.scrollHeight - top - viewport.clientHeight,
      near: NEAR_BOTTOM,
    })
    programmatic = false
    lastTop = top
  }

  /** Scrolls so row `i` is in view, and says where it is when the list has not drawn it yet. */
  export function scrollToRow(i: number) {
    if (!viewport || i < 0 || i >= count) return
    const [from, to] = [offsets[i], offsets[i + 1]]
    if (from < viewport.scrollTop) place(from)
    else if (to > viewport.scrollTop + viewport.clientHeight) place(to - viewport.clientHeight)
  }

  export function toBottom() {
    follow = true
    if (viewport) place(viewport.scrollHeight)
  }

  // New or taller content while following: stay at the bottom.
  $effect(() => {
    void total
    if (stick && follow && viewport) place(viewport.scrollHeight)
  })

  $effect(() => {
    const [a, b] = range
    if (onrange && b > a) untrack(() => onrange(a, b - 1))
  })
</script>

<svelte:window onpointerup={() => (held = false)} onpointercancel={() => (held = false)} />

<div class="wrap">
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions: a scroll region takes focus so the keyboard can scroll it, and the listeners only note that the user is scrolling -->
  <div class="viewport" role="region" aria-label={label} tabindex="0" bind:this={viewport} bind:clientHeight={height} onscroll={onScroll} onwheel={touched} ontouchstart={touched} ontouchmove={touched} onkeydown={touched} onpointerdown={() => (held = true)}>
    <div class="space" style:height="{total}px">
      <div class="rows" style:transform="translateY({offsets[range[0]]}px)">
        {#each rows as i (i)}
          <div class="row" data-i={i} use:measure>{@render row(i)}</div>
        {/each}
      </div>
    </div>
  </div>
  {#if jump && !follow && count > 0 && scrollable}
    <button type="button" class="btn latest" onclick={toBottom}>{jump}</button>
  {/if}
</div>

<style>
  .wrap {
    position: relative;
    flex: 1;
    min-height: 0;
  }
  .viewport {
    height: 100%;
    overflow-y: auto;
    overflow-anchor: none;
    overscroll-behavior: contain;
  }
  .space {
    position: relative;
    overflow: hidden;
  }
  .rows {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
  }
  .latest {
    position: absolute;
    right: var(--s-4);
    bottom: var(--s-3);
    background: var(--raised);
  }
</style>
