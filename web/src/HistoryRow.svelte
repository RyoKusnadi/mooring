<script lang="ts">
  // One commit in the list: lanes, ref labels, subject, author and time. The row is a button that opens the commit.
  import HistoryGraph from './HistoryGraph.svelte'
  import type { GraphRow } from './lib/graph'
  import { refWord, relativeTime, shortId, type Commit } from './lib/historyapi'

  let {
    commit,
    graph,
    lanes,
    index,
    current,
    tab,
    now,
    onpick,
  }: { commit: Commit; graph?: GraphRow; lanes: number; index: number; current: boolean; tab: boolean; now: number; onpick: (i: number) => void } = $props()

  const SHOWN = 3
  const refs = $derived(commit.refs.slice(0, SHOWN))
  const more = $derived(commit.refs.slice(SHOWN))
</script>

<button type="button" class="row" class:current data-hrow={index} tabindex={tab ? 0 : -1} aria-current={current ? 'true' : undefined} onclick={() => onpick(index)}>
  {#if graph && lanes > 0}<HistoryGraph row={graph} {lanes} />{/if}
  <span class="main">
    <span class="top">
      {#each refs as r (r.kind + r.name)}
        <span class="ref {r.kind}"><span class="sr">{refWord(r.kind)} </span>{r.kind === 'head' && r.name !== 'HEAD' ? `HEAD → ${r.name}` : r.name}</span>
      {/each}
      {#if more.length > 0}<span class="ref more" title={more.map((r) => `${r.name} (${refWord(r.kind)})`).join(', ')}>+{more.length} more</span>{/if}
      <span class="subject">{commit.subject || '(no message)'}</span>
    </span>
    <span class="meta"><span class="who">{commit.author}</span><span class="id">{shortId(commit.id)}</span><time datetime={new Date(commit.time * 1000).toISOString()}>{relativeTime(commit.time, now)}</time></span>
  </span>
</button>

<style>
  .row {
    display: flex;
    align-items: stretch;
    gap: var(--s-2);
    width: 100%;
    min-height: 32px;
    padding: 0 var(--s-3);
    border: 0;
    border-left: 3px solid transparent;
    border-radius: 0;
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  .row:hover {
    background: var(--hover);
  }
  .row.current {
    background: var(--selected);
    border-left-color: var(--accent);
  }
  .main {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-3);
    min-width: 0;
  }
  .top {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
  }
  .subject {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta {
    display: flex;
    flex: none;
    gap: var(--s-3);
    color: var(--muted);
    font-size: var(--fs-s);
    white-space: nowrap;
  }
  .who {
    max-width: 16ch;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .id {
    font-family: var(--font-mono);
  }
  .ref {
    flex: none;
    max-width: 22ch;
    padding: 0 var(--s-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    overflow: hidden;
    font-size: var(--fs-s);
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Four kinds told apart by fill, edge style and a word for screen readers, not by colour. */
  .ref.head {
    border-color: var(--accent);
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
  }
  .ref.remote {
    border-style: dashed;
    color: var(--muted);
  }
  .ref.tag {
    border-left-width: 4px;
    font-style: italic;
  }
  .ref.more {
    border-style: dotted;
    color: var(--muted);
  }
  @media (max-width: 900px) {
    .row {
      min-height: 48px;
      padding: 0 var(--s-2);
    }
    /* The vertical padding sits on the text, not the row, so the lane lines run from edge to edge. */
    .main {
      padding-block: var(--s-1);
      flex-direction: column;
      align-items: stretch;
      justify-content: center;
      gap: 0;
    }
    .top {
      flex: none;
    }
    .meta {
      gap: var(--s-2);
    }
    .id {
      display: none;
    }
  }
  @media (pointer: coarse) {
    .row {
      min-height: 52px;
    }
  }
</style>
