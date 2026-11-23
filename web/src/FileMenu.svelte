<script lang="ts">
  import { tick } from 'svelte'
  import { floatMenu } from './lib/floatmenu'

  let { label, items }: { label: string; items: { label: string; run: () => void; danger?: boolean }[] } = $props()

  let shown = $state(false)
  let root: HTMLElement | undefined = $state()
  let trigger: HTMLButtonElement | undefined = $state()
  let menu: HTMLElement | undefined = $state()

  export async function open() {
    shown = true
    await tick()
    menu?.querySelector<HTMLElement>('button')?.focus({ preventScroll: true })
  }

  function close(refocus = true) {
    shown = false
    if (refocus) trigger?.focus()
  }

  function keys(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation()
      return close()
    }
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const all = [...menu!.querySelectorAll<HTMLElement>('button')]
    const i = all.indexOf(document.activeElement as HTMLElement)
    all[(i + step + all.length) % all.length]?.focus()
  }
</script>

<svelte:window onpointerdown={(e) => shown && !root?.contains(e.target as Node) && close(false)} />

<span class="wrap" bind:this={root}>
  <button type="button" class="more" bind:this={trigger} aria-label={label} tabindex="-1" aria-haspopup="menu" aria-expanded={shown} onclick={() => (shown ? close() : open())}>…</button>
  {#if shown}
    <div class="menu" role="menu" aria-label={label} tabindex="-1" bind:this={menu} onkeydown={keys} use:floatMenu={{ anchor: () => trigger, close: () => close() }} onfocusout={(e) => !root?.contains(e.relatedTarget as Node) && close(false)}>
      {#each items as item (item.label)}
        <button type="button" role="menuitem" class:danger={item.danger} onclick={() => (close(false), item.run())}>{item.label}</button>
      {/each}
    </div>
  {/if}
</span>

<style>
  .wrap {
    position: relative;
    flex: none;
  }
  .more {
    width: var(--control-h);
    min-height: var(--control-h);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--muted);
    font: inherit;
    opacity: 0;
  }
  .more[aria-expanded='true'] {
    opacity: 1;
  }
  .more:hover {
    color: var(--fg);
  }
  .menu {
    min-width: 160px;
  }
  @media (pointer: coarse) {
    .more {
      opacity: 1;
    }
  }
</style>
