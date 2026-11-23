<script lang="ts">
  // The menu of one tab (right click, the menu key, or Shift+F10): close, copy path, reveal, pin.
  import { onMount, tick } from 'svelte'
  import { actions } from './lib/actions'
  import { files } from './lib/filestore.svelte'
  import { joinPath } from './lib/paths'
  import { reveal } from './lib/reveal'
  import { app } from './lib/state.svelte'
  import type { Tab } from './lib/layout'
  import { othersOf, rightOf } from './lib/tabedit'

  let { tab, x, y, onClose }: { tab: Tab; x: number; y: number; onClose: () => void } = $props()

  let menu: HTMLElement | undefined = $state()
  let at = $state({ left: 0, top: 0 })

  const others = $derived(app.layout ? othersOf(app.layout, tab.id) : [])
  const right = $derived(app.layout ? rightOf(app.layout, tab.id) : [])
  const isFile = $derived(tab.kind === 'file' && tab.path !== undefined)
  const hasPath = $derived(isFile || (tab.kind === 'rfile' && tab.path !== undefined))

  /** Closes the tabs without unsaved edits. The first one with edits asks (save, discard or keep), as a single close does; the rest stay open. */
  function closeMany(ts: Tab[]) {
    const edited = ts.filter((t) => files.dirty[t.id])
    ts.filter((t) => !files.dirty[t.id]).forEach((t) => actions.closeTab(t.id))
    if (edited.length) {
      actions.closeTab(edited[0].id)
      if (edited.length > 1) app.notice = `${edited.length} tabs have unsaved changes. Answer each one to close it.`
    }
  }

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text)
      app.info = 'Copied the path.'
      const mine = app.info
      setTimeout(() => app.info === mine && (app.info = ''), 4000)
    } catch {
      app.notice = 'Could not copy. The browser did not allow it.'
    }
  }

  const items = $derived([
    { label: 'Close', run: () => actions.closeTab(tab.id), show: true },
    { label: 'Close others', run: () => closeMany(others), show: others.length > 0 },
    { label: 'Close to the right', run: () => closeMany(right), show: right.length > 0 },
    { label: 'Copy path', run: () => copy(tab.path!), show: hasPath },
    { label: 'Copy full path', run: () => copy(joinPath(app.current?.path ?? '', tab.path!)), show: isFile },
    { label: 'Reveal in files', run: () => void reveal(tab.path!), show: isFile },
    { label: tab.pinned ? 'Unpin' : 'Pin', run: () => actions.pinTab(tab.id, !tab.pinned), show: true },
  ].filter((i) => i.show))

  // The command runs while the menu still exists: it reads this tab's fields.
  function finish(run?: () => void) {
    run?.()
    onClose()
  }

  onMount(() => {
    void tick().then(() => {
      const r = menu?.getBoundingClientRect()
      at = { left: Math.max(4, Math.min(x, innerWidth - (r?.width ?? 0) - 4)), top: Math.max(4, Math.min(y, innerHeight - (r?.height ?? 0) - 4)) }
      menu?.querySelector<HTMLElement>('button')?.focus()
    })
  })

  function keys(e: KeyboardEvent) {
    if (e.key === 'Escape' || e.key === 'Tab') {
      e.preventDefault()
      e.stopPropagation()
      return finish()
    }
    const all = [...menu!.querySelectorAll<HTMLElement>('button')]
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

<svelte:window onpointerdown={(e) => !menu?.contains(e.target as Node) && onClose()} />

<div class="menu" role="menu" aria-label="Tab {tab.title}" tabindex="-1" bind:this={menu} style:left="{at.left}px" style:top="{at.top}px" onkeydown={keys}>
  {#each items as item (item.label)}
    <button type="button" role="menuitem" onclick={() => finish(item.run)}>{item.label}</button>
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    z-index: 20;
  }
</style>
