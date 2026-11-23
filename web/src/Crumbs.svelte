<script lang="ts">
  // The path of the open file as folder names. Each folder shows itself in the file tree; the last name is the file.
  import { reveal } from './lib/reveal'
  import type { SymbolStep } from './lib/lspsymbols'

  /** `symbols` are the code symbols at the cursor, outermost first; empty when the server lists none. */
  let { path, symbols = [], onSymbol }: { path: string; symbols?: SymbolStep[]; onSymbol?: (s: SymbolStep) => void } = $props()

  const parts = $derived(path.split('/').filter(Boolean))
  const folders = $derived(parts.slice(0, -1))
  let strip: HTMLElement | undefined = $state()
  let cut = $state(false)

  // The folders scroll so the one next to the file name stays in view, and a fade says the start is cut.
  // The strip changes width when the buttons next to it come and go, so this runs on every resize.
  const toEnd = () => {
    if (!strip) return
    strip.scrollLeft = strip.scrollWidth
    cut = strip.scrollLeft > 0
  }
  $effect(() => {
    void path
    toEnd()
    if (!strip) return
    const watch = new ResizeObserver(toEnd)
    watch.observe(strip)
    return () => watch.disconnect()
  })
</script>

<nav class="crumbs" aria-label="Path of {path}" title={path}>
  <ol class="dirs" class:cut bind:this={strip} onscroll={() => strip && (cut = strip.scrollLeft > 0)}>
    {#each folders as name, i (i)}
      <li><button type="button" onclick={() => reveal(folders.slice(0, i + 1).join('/'), true)} aria-label="Show folder {folders.slice(0, i + 1).join('/')} in the file tree">{name}</button></li>
    {/each}
  </ol>
  <span class="file" aria-current="page">{parts[parts.length - 1] ?? path}</span>
  {#if symbols.length}
    <ol class="syms" aria-label="Symbols at the cursor">
      {#each symbols as step (step.line + ':' + step.character)}
        <li><button type="button" title="Go to {step.name}" onclick={() => onSymbol?.(step)}>{step.name}</button></li>
      {/each}
    </ol>
  {/if}
</nav>

<style>
  .crumbs {
    display: flex;
    flex: 1 1 auto;
    align-items: center;
    max-width: 100%;
    min-width: 0;
    font: var(--fs-s) var(--font-mono);
    white-space: nowrap;
  }
  /* The folders give way first; the file name keeps its width unless it alone is wider than the strip. */
  .dirs {
    display: flex;
    flex: 0 1000 auto;
    align-items: center;
    min-width: 0;
    margin: 0;
    padding: 0;
    overflow-x: auto;
    overflow-y: hidden;
    list-style: none;
    scrollbar-width: none;
  }
  .dirs.cut {
    mask-image: linear-gradient(to right, transparent, #000 var(--s-5));
  }
  li::after {
    content: '/';
    padding: 0 var(--s-1);
    color: var(--muted);
  }
  /* Symbols give way before the folders and never touch the file name. */
  .syms {
    display: flex;
    flex: 0 2000 auto;
    align-items: center;
    min-width: 0;
    margin: 0;
    padding: 0;
    overflow: hidden;
    list-style: none;
  }
  .syms li {
    display: flex;
    min-width: 0;
  }
  .syms li::before {
    content: '\203A';
    padding: 0 var(--s-1);
    color: var(--muted);
  }
  .syms li::after {
    content: none;
  }
  .syms button {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  button {
    min-height: var(--control-h);
    padding: 0 var(--s-1);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--muted);
    font: inherit;
  }
  button:hover {
    background: var(--hover);
    color: var(--fg);
  }
  .file {
    flex: 0 0 auto;
    max-width: 100%;
    overflow: hidden;
    font-weight: var(--fw-semibold);
    text-overflow: ellipsis;
  }
  @media (pointer: coarse) {
    button {
      min-height: 44px;
      padding: 0 var(--s-2);
    }
  }
</style>
