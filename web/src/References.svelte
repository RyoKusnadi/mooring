<script lang="ts">
  // "Find references": every place the language server says the name under the cursor is used.
  import { onMount, tick, untrack } from 'svelte'
  import SshModal from './SshModal.svelte'
  import { fileActions } from './lib/fileactions'
  import type { Ref } from './lib/lsprefactor'
  import type { RefactorRequest } from './lib/refactorview.svelte'
  import { groupByPath, markName, refsSummary, MAX_READ } from './lib/refsview'

  let { request, onClose }: { request: RefactorRequest; onClose: (focus?: boolean) => void } = $props()

  // One dialog serves one request; a new request mounts a new dialog.
  const { view, projectId, projectPath } = untrack(() => request)
  const word = view.state.wordAt(view.state.selection.main.head)
  const name = word ? view.state.sliceDoc(word.from, word.to) : ''

  let result = $state<{ refs: Ref[] } | { error: string } | null>(null)
  let root: HTMLElement | undefined = $state()
  const groups = $derived(result && 'refs' in result ? groupByPath(result.refs) : [])

  onMount(async () => {
    try {
      const m = await import('./lib/lspbind')
      result = await m.findReferences(view, projectId, projectPath)
    } catch {
      result = { error: 'The language server could not find references.' }
    }
    await tick()
    // Rows first; with none, the Close button.
    ;(root?.querySelector<HTMLElement>('button') ?? document.querySelector<HTMLElement>('dialog[open] [data-primary]'))?.focus()
  })

  function open(r: Ref) {
    fileActions.openFile(r.path, r.line)
    onClose(false)
  }
</script>

<SshModal title={name ? `References to ${name}` : 'References'} wide {onClose}>
  <div bind:this={root}>
    {#if !result}
      <p class="muted" role="status" data-first tabindex="-1">Asking the language server…</p>
    {:else if 'error' in result}
      <p class="error" role="alert" data-first tabindex="-1">{result.error}</p>
    {:else if result.refs.length === 0}
      <p class="muted" role="status" data-first tabindex="-1">The language server found no references. Put the cursor on a name and try again.</p>
    {:else}
      <p class="muted" role="status">{refsSummary(result.refs.length, groups.length)}. Click a row to open the file at that line.{#if groups.length > MAX_READ} Lines are shown for the first {MAX_READ} files only.{/if}</p>
      <div class="list">
        {#each groups as g (g.path)}
          <section aria-label={g.path}>
            <h3><span class="path">{g.path}</span><span class="count">{g.items.length}</span></h3>
            <ul>
              {#each g.items as r (r.line + ':' + r.col)}
                <li>
                  <button type="button" class="row" aria-label="{r.path}:{r.line} {r.text}" onclick={() => open(r)}>
                    <span class="line">{r.line}</span>
                    <span class="text">{#if r.text}{#each markName(r.text, name) as s, k (k)}{#if s.hit}<mark>{s.text}</mark>{:else}{s.text}{/if}{/each}{:else}<span class="unread">line not read</span>{/if}</span>
                  </button>
                </li>
              {/each}
            </ul>
          </section>
        {/each}
      </div>
    {/if}
  </div>
  {#snippet footer()}
    <button type="button" class="btn" data-primary onclick={() => onClose()}>Close</button>
  {/snippet}
</SshModal>

<style>
  .muted,
  .error {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-s);
  }
  .muted {
    color: var(--muted);
  }
  .error {
    color: var(--danger);
  }
  .list {
    max-height: 55dvh;
    overflow-y: auto;
  }
  h3 {
    display: flex;
    gap: var(--s-2);
    margin: var(--s-2) 0 0;
    padding: 0 var(--s-2);
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .path {
    min-width: 0;
    font-family: var(--font-mono);
    overflow-wrap: anywhere;
  }
  .count {
    flex: none;
    color: var(--muted);
    font-weight: var(--fw-regular);
  }
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .row {
    display: grid;
    grid-template-columns: 4ch minmax(0, 1fr);
    gap: var(--s-2);
    align-items: baseline;
    width: 100%;
    min-height: var(--control-h);
    padding: var(--s-1) var(--s-2);
    border: 0;
    border-radius: var(--r-s);
    background: transparent;
    color: var(--fg);
    font: var(--fs-s) var(--font-mono);
    text-align: left;
  }
  .row:hover {
    background: var(--hover);
  }
  .line {
    color: var(--muted);
    text-align: right;
  }
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: pre;
  }
  .unread {
    color: var(--muted);
  }
  mark {
    background: var(--attention-bg);
    color: var(--attention);
    font-weight: var(--fw-semibold);
    outline: 1px solid var(--attention);
  }
  @media (pointer: coarse), (max-width: 600px) {
    .row {
      min-height: 44px;
    }
  }
</style>
