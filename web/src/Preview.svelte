<script lang="ts">
  // A file drawn instead of shown as text: markdown, a CSV or TSV table, or an HTML page. Loaded the
  // first time a tab switches to Preview, so none of it is in the app shell.
  import Markdown from './Markdown.svelte'
  import { delimiterFor, parseCsv } from './lib/csv'
  import type { PreviewKind } from './lib/previewtype'

  let { kind, text, projectId, path }: { kind: PreviewKind; text: string; projectId: string; path: string } = $props()

  const MAX_ROWS = 1001
  const csv = $derived(kind === 'csv' ? parseCsv(text, delimiterFor(path), MAX_ROWS) : null)
  // Keeps the frame locked even if the app's own policy is loosened later.
  const LOCK = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">`
  const empty = $derived(text.trim() === '')
  const head = $derived(csv?.rows[0] ?? [])
  const body = $derived(csv?.rows.slice(1) ?? [])
</script>

{#if empty}
  <p class="note solo">This file is empty.</p>
{:else if kind === 'markdown'}
  <div class="scroll"><div class="doc"><Markdown {text} {projectId} /></div></div>
{:else if kind === 'csv'}
  <div class="scroll">
    {#if !csv?.rows.length}
      <p class="note">This file is empty.</p>
    {:else}
      <table aria-label="Table of {path}">
        <thead><tr>{#each head as h, i (i)}<th scope="col">{h}</th>{/each}</tr></thead>
        <tbody>{#each body as r, i (i)}<tr>{#each r as c, j (j)}<td>{c}</td>{/each}</tr>{/each}</tbody>
      </table>
      {#if csv.more}<p class="note" role="status">Showing the header and the first {MAX_ROWS - 1} rows. Switch to Source to see the whole file.</p>{/if}
    {/if}
  </div>
{:else}
  <p class="note">Scripts do not run in this preview, and links and the page's own files do not load.</p>
  <iframe title="Preview of {path}" sandbox="" srcdoc={LOCK + text}></iframe>
{/if}

<style>
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-4);
  }
  .doc {
    max-width: 90ch;
  }
  .note {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .solo {
    padding: var(--s-4);
  }
  .scroll > .note {
    padding-left: 0;
  }
  table {
    border-collapse: collapse;
    font-size: var(--fs-m);
    line-height: 20px;
  }
  th,
  td {
    max-width: 40ch;
    padding: var(--s-1) var(--s-3);
    border: 1px solid var(--border);
    overflow-wrap: anywhere;
    text-align: left;
    vertical-align: top;
    white-space: pre-wrap;
  }
  th {
    position: sticky;
    top: calc(-1 * var(--s-4));
    background: var(--raised);
  }
  iframe {
    flex: 1;
    min-height: 0;
    width: 100%;
    border: 0;
    border-top: 1px solid var(--border);
    background: Canvas;
    color-scheme: light;
  }
</style>
