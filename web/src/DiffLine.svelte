<script lang="ts">
  // One row of a diff: a hunk header or a line. When lines can be chosen, a changed line is a checkbox
  // (click, drag, or Space) and a hunk header carries its own "Stage block" button.
  import type { DiffRow } from './lib/diffrows'

  let {
    row,
    i,
    choosable = false,
    chosen = false,
    tab = false,
    staged = false,
    busy = false,
    ondown,
    onenter,
    onclick,
    onkey,
    onblock,
  }: {
    row: DiffRow
    i: number
    choosable?: boolean
    chosen?: boolean
    tab?: boolean
    staged?: boolean
    busy?: boolean
    ondown?: (i: number, e: PointerEvent) => void
    onenter?: (i: number) => void
    onclick?: (i: number) => void
    onkey?: (i: number, e: KeyboardEvent) => void
    onblock?: (i: number) => void
  } = $props()

  const changed = $derived(row.type === 'line' && (row.kind === '+' || row.kind === '-'))
</script>

{#if row.type === 'hunk'}
  <div class="hunk">
    <span class="what">{row.text}</span>
    {#if choosable}
      <button type="button" class="btn small" disabled={busy} aria-label={`${staged ? 'Unstage' : 'Stage'} block: ${row.text}`} onclick={() => onblock?.(i)}>{staged ? 'Unstage block' : 'Stage block'}</button>
    {/if}
  </div>
{:else if choosable && changed}
  <div
    class="line pick"
    class:add={row.kind === '+'}
    class:del={row.kind === '-'}
    class:chosen
    role="checkbox"
    aria-checked={chosen}
    tabindex={tab ? 0 : -1}
    data-row={i}
    onpointerdown={(e) => ondown?.(i, e)}
    onpointerenter={() => onenter?.(i)}
    onclick={() => onclick?.(i)}
    onkeydown={(e) => onkey?.(i, e)}
  >
    <span class="box" aria-hidden="true">{chosen ? '✓' : ''}</span>
    <span class="n" aria-hidden="true">{row.old ?? ''}</span>
    <span class="n" aria-hidden="true">{row.now ?? ''}</span>
    <span class="mark" aria-hidden="true">{row.kind === '+' ? '+' : '−'}</span>
    <span class="text"><span class="sr">{row.kind === '+' ? 'Added: ' : 'Removed: '}</span>{row.text}</span>
  </div>
{:else}
  <div class="line" class:add={row.kind === '+'} class:del={row.kind === '-'} class:eof={row.kind === '\\'} class:plain={choosable}>
    <span class="n" aria-hidden="true">{row.old ?? ''}</span>
    <span class="n" aria-hidden="true">{row.now ?? ''}</span>
    <span class="mark" aria-hidden="true">{row.kind === '+' ? '+' : row.kind === '-' ? '−' : ''}</span>
    <span class="text">{#if row.kind === '+'}<span class="sr">Added: </span>{:else if row.kind === '-'}<span class="sr">Removed: </span>{/if}{row.text}</span>
  </div>
{/if}

<style>
  .hunk {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    padding: 2px var(--s-3);
    background: var(--surface);
    border-block: 1px solid var(--border);
    color: var(--muted);
    font: var(--fs-s) / 1.5 var(--font-mono);
  }
  .what {
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .small {
    flex: none;
    min-height: calc(var(--control-h) - 6px);
    padding: 0 var(--s-2);
    background: var(--raised);
    font: 600 var(--fs-s) var(--font-ui);
  }
  .line {
    display: grid;
    grid-template-columns: 5ch 5ch 2ch minmax(0, 1fr);
    gap: 0 var(--s-2);
    padding: 0 var(--s-3);
    font: var(--fs-s) / 1.6 var(--font-mono);
  }
  .line.pick {
    grid-template-columns: 2.4ch 5ch 5ch 2ch minmax(0, 1fr);
    cursor: pointer;
  }
  .line.plain {
    padding-left: calc(var(--s-3) + 2.4ch + var(--s-2));
  }
  .add {
    background: var(--diff-add);
  }
  .del {
    background: var(--diff-del);
  }
  .pick.chosen {
    background: var(--selected);
  }
  .pick:focus-visible {
    outline-offset: -2px;
  }
  .box {
    align-self: center;
    justify-self: center;
    width: 2.2ch;
    height: 2.2ch;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-s);
    background: var(--raised);
    color: var(--accent);
    font-weight: 600;
    line-height: 2ch;
    text-align: center;
  }
  .chosen .box {
    border-color: var(--accent);
  }
  .n {
    color: var(--muted);
    text-align: right;
    user-select: none;
  }
  .mark {
    font-weight: 600;
    text-align: center;
    user-select: none;
  }
  .add .mark {
    color: var(--success);
  }
  .del .mark {
    color: var(--danger);
  }
  .text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .eof .text {
    color: var(--muted);
    font-style: italic;
  }
  @media (max-width: 900px) {
    .line {
      grid-template-columns: 4ch 4ch 2ch minmax(0, 1fr);
      gap: 0 var(--s-1);
      padding: 0 var(--s-2);
    }
    .line.pick {
      grid-template-columns: 2.4ch 4ch 4ch 2ch minmax(0, 1fr);
    }
    .line.plain {
      padding-left: calc(var(--s-2) + 2.4ch + var(--s-1));
    }
  }
  @media (pointer: coarse) {
    .line.pick {
      min-height: 44px;
      align-items: center;
    }
    .box {
      width: 26px;
      height: 26px;
      line-height: 24px;
    }
    .line.pick {
      grid-template-columns: 30px 4ch 4ch 2ch minmax(0, 1fr);
    }
    .small {
      min-height: 44px;
    }
  }
</style>
