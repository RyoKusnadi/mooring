<script lang="ts">
  // The rows of the finder: file names with their folders, or matching lines with the match marked.
  import type { Match } from './lib/filesapi'
  import { segments } from './lib/fuzzy'
  import { baseName, parentPath } from './lib/paths'

  type Row = { id: string; path: string; line: number; match: Match | null }
  let { rows, current, mode, query, indexing, onopen, onhover }: { rows: Row[]; current: number; mode: 'file' | 'text'; query: string; indexing: boolean; onopen: (r: Row) => void; onhover: (i: number) => void } = $props()
</script>

<ul id="find-list" role="listbox" aria-label={mode === 'file' ? 'Files' : 'Matches'}>
  {#each rows as row, i (row.id)}
    <!-- Keyboard selection happens in the input; the rows only need the pointer. -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <li id={'find-' + row.id} role="option" aria-selected={i === current} onclick={() => onopen(row)} onpointermove={() => onhover(i)}>
      {#if row.match}
        <span class="where">{row.path}:{row.line}</span>
        <span class="text">{#each segments(row.match.text, row.match.ranges) as s, k (k)}{#if s.hit}<mark>{s.text}</mark>{:else}{s.text}{/if}{/each}</span>
      {:else}
        <span class="base">{baseName(row.path)}</span>
        <span class="dir">{parentPath(row.path)}</span>
      {/if}
    </li>
  {:else}
    {#if mode === 'file' && query && !indexing}<li class="none" role="presentation">No file name matches “{query}”.</li>{/if}
  {/each}
</ul>

<style>
  ul {
    flex: 1;
    min-height: 0;
    margin: 0;
    padding: 0 var(--s-1) var(--s-1);
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0 var(--s-3);
    min-height: var(--control-h);
    padding: var(--s-1) var(--s-2);
    border-radius: var(--r-s);
    cursor: pointer;
  }
  li[aria-selected='true'] {
    background: var(--selected);
    box-shadow: inset 2px 0 var(--accent);
  }
  .none {
    color: var(--muted);
    cursor: default;
  }
  .base {
    font-weight: 600;
  }
  .dir,
  .where {
    min-width: 0;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .where {
    flex: none;
    max-width: 100%;
    font-family: var(--font-mono);
  }
  .text {
    flex: 1 1 100%;
    min-width: 0;
    overflow: hidden;
    font: var(--fs-s) var(--font-mono);
    text-overflow: ellipsis;
    white-space: pre;
  }
  mark {
    background: var(--attention-bg);
    color: var(--attention);
    font-weight: 600;
    outline: 1px solid var(--attention);
  }
</style>
