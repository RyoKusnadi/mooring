<script lang="ts">
  // A reply as markdown. Finished blocks are drawn once; only the last block is drawn again per chunk.
  import { untrack } from 'svelte'
  import { MdStream } from './lib/mdstream'
  import { renderBlocks, type MdContext } from './lib/mddom'
  import { openProjectFile } from './lib/chatlinks'
  import { app } from './lib/state.svelte'

  let { text, projectId = '' }: { text: string; projectId?: string } = $props()

  let host: HTMLDivElement | undefined = $state()
  const stream = new MdStream()
  let drawn = 0
  let tail: HTMLElement | null = null

  const ctx: MdContext = {
    get root() {
      return app.projects.find((p) => p.id === projectId)?.path ?? ''
    },
    openFile: (path, line) => void openProjectFile(projectId, path, line),
  }

  $effect(() => {
    const t = text
    const box = host
    if (!box) return
    untrack(() => {
      stream.update(t)
      const done = stream.final
      if (done.length < drawn) {
        box.replaceChildren()
        drawn = 0
        tail = null
      }
      tail?.remove()
      const frag = document.createDocumentFragment()
      for (; drawn < done.length; drawn++) renderBlocks(done[drawn].blocks, ctx, frag)
      tail = document.createElement('div')
      tail.className = 'md-tail'
      renderBlocks(stream.tail.blocks, ctx, tail)
      box.append(frag, tail)
    })
  })
</script>

<div class="md" bind:this={host}></div>

<style>
  .md {
    font-size: var(--fs-read);
    line-height: var(--lh-read);
    overflow-wrap: anywhere;
  }
  .md :global(.md-tail) {
    display: contents;
  }
  .md > :global(*),
  .md :global(.md-tail > *) {
    margin: 0 0 var(--s-3);
  }
  .md :global(.md-tail > :last-child),
  .md > :global(:last-child:not(.md-tail)) {
    margin-bottom: 0;
  }
  .md :global(.md-h) {
    margin-top: var(--s-4);
    font-size: var(--fs-read);
    font-weight: var(--fw-semibold);
    line-height: var(--lh-read);
  }
  .md :global(h3.md-h),
  .md :global(h4.md-h) {
    font-size: var(--fs-l);
    line-height: 1.4;
  }
  .md :global(ul),
  .md :global(ol) {
    padding-left: var(--s-5);
  }
  .md :global(li > *) {
    margin: 0 0 var(--s-1);
  }
  .md :global(li > :last-child) {
    margin-bottom: 0;
  }
  .md :global(li.md-task) {
    list-style: none;
    margin-left: calc(var(--s-5) * -1);
  }
  .md :global(li.md-task > input) {
    margin: 0 var(--s-2) 0 0;
    vertical-align: middle;
  }
  .md :global(li.md-task > p) {
    display: inline;
  }
  .md :global(blockquote) {
    padding-left: var(--s-3);
    border-left: 2px solid var(--border-strong);
    color: var(--muted);
  }
  .md :global(hr) {
    border: 0;
    border-top: 1px solid var(--border);
  }
  .md :global(.md-code) {
    padding: 0 var(--s-1);
    border-radius: var(--r-s);
    background: var(--surface);
    font: var(--fs-code) / 1 var(--font-mono);
  }
  .md :global(.md-link) {
    padding: 0;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;
  }
  .md :global(.md-link:hover) {
    color: var(--accent-hover);
  }
  .md :global(.md-pre) {
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background: var(--surface);
    overflow: hidden;
  }
  .md :global(.md-pre-bar) {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: var(--row-h);
    padding: 0 var(--s-1) 0 var(--s-3);
    border-bottom: 1px solid var(--border);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .md :global(.md-pre pre) {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    overflow-x: auto;
    font: var(--fs-code) / var(--lh-code) var(--font-mono);
  }
  .md :global(.md-pre code) {
    font: inherit;
    white-space: pre;
  }
  .md :global(.md-table) {
    overflow-x: auto;
  }
  .md :global(table) {
    width: max-content;
    max-width: none;
    border-collapse: collapse;
    font-size: var(--fs-m);
    line-height: var(--lh-m);
  }
  .md :global(th),
  .md :global(td) {
    padding: var(--s-1) var(--s-3);
    border: 1px solid var(--border);
    overflow-wrap: normal;
    text-align: left;
    vertical-align: top;
  }
  .md :global(th) {
    background: var(--surface);
    font-weight: var(--fw-semibold);
  }
  .md :global(.tk-kw) {
    color: var(--ansi-5);
  }
  .md :global(.tk-str) {
    color: var(--ansi-2);
  }
  .md :global(.tk-num) {
    color: var(--ansi-4);
  }
  .md :global(.tk-cm) {
    color: var(--muted);
    font-style: italic;
  }
  .md :global(.tk-ty) {
    color: var(--ansi-6);
  }
  .md :global(.tk-fn) {
    color: var(--fg);
    font-weight: var(--fw-semibold);
  }
  .md :global(.tk-pr) {
    color: var(--ansi-3);
  }
</style>
