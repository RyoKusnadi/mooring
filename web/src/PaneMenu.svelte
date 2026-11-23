<script lang="ts">
  // A button that opens a small menu of commands: "+" for new things, "…" for the rest of the pane actions.
  import { tick } from 'svelte'
  import Icon from './Icon.svelte'
  import type { IconName } from './lib/icons'

  export interface Entry {
    label: string
    icon: IconName
    hint?: string
    disabled?: boolean
    run: () => void
  }

  let { label, icon, items, mark = '' }: { label: string; icon: IconName; items: Entry[]; mark?: string } = $props()

  let shown = $state(false)
  let root: HTMLElement | undefined = $state()
  let trigger: HTMLButtonElement | undefined = $state()
  let menu: HTMLElement | undefined = $state()

  async function open() {
    shown = true
    await tick()
    menu?.querySelector<HTMLElement>('button:not(:disabled)')?.focus()
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
    const all = [...menu!.querySelectorAll<HTMLElement>('button:not(:disabled)')]
    if (e.key === 'Home' || e.key === 'End') {
      e.preventDefault()
      return all[e.key === 'Home' ? 0 : all.length - 1]?.focus()
    }
    const step = e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0
    if (!step) return
    e.preventDefault()
    const i = all.indexOf(document.activeElement as HTMLElement)
    all[(i + step + all.length) % all.length]?.focus()
  }
</script>

<svelte:window onpointerdown={(e) => shown && !root?.contains(e.target as Node) && close(false)} />

<span class="wrap" bind:this={root}>
  <button type="button" class="btn ghost icon" bind:this={trigger} aria-label={label} title={label} aria-haspopup="menu" aria-expanded={shown} data-plus={mark || undefined} onclick={() => (shown ? close() : open())} onkeydown={(e) => e.key === 'ArrowDown' && !shown && (e.preventDefault(), open())}><Icon name={icon} /></button>
  {#if shown}
    <div class="menu" role="menu" aria-label={label} tabindex="-1" bind:this={menu} onkeydown={keys} onfocusout={(e) => !root?.contains(e.relatedTarget as Node) && close(false)}>
      {#each items as item (item.label)}
        <button type="button" role="menuitem" disabled={item.disabled} onclick={() => (close(false), item.run())}>
          <Icon name={item.icon} /><span>{item.label}</span>{#if item.hint}<kbd>{item.hint}</kbd>{/if}
        </button>
      {/each}
    </div>
  {/if}
</span>

<style>
  .wrap {
    position: relative;
    flex: none;
  }
  .icon {
    width: var(--control-h);
    padding: 0;
    color: var(--muted);
  }
  .icon:hover:not(:disabled) {
    color: var(--fg);
  }
  .menu {
    position: absolute;
    z-index: 5;
    top: 100%;
    right: 0;
  }
  kbd {
    margin-left: auto;
    padding-left: var(--s-4);
    color: var(--muted);
    font: var(--fs-s) var(--font-ui);
  }
</style>
