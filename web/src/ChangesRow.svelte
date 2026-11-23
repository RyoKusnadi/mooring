<script lang="ts">
  // One file in the Changes list: letter, the name in full, the dimmed folder, then Stage and Discard.
  import Icon from './Icon.svelte'
  import StatusBadge from './StatusBadge.svelte'
  import { sideLetter, sideWord, type Change } from './lib/gitapi'
  import { baseName } from './lib/paths'

  let { c, staged, picked, key, tab, busy, detail, onfocus, onopen, onkeys, ontoggle, ondiscard, labels, onresolve }: {
    c: Change
    staged: boolean
    picked: boolean
    key: string
    tab: boolean
    busy: boolean
    detail: string
    onfocus: () => void
    onopen: () => void
    onkeys: (e: KeyboardEvent) => void
    ontoggle: () => void
    ondiscard: () => void
    labels: { ours: string; theirs: string }
    onresolve: (side: 'ours' | 'theirs' | 'mark') => void
  } = $props()
</script>

<li class:picked data-row={key}>
  <button
    type="button"
    class="open"
    data-key={key}
    tabindex={tab ? 0 : -1}
    title={c.path}
    aria-label={`${c.path}, ${sideWord(c, staged)}. Show changes`}
    aria-current={picked ? 'true' : undefined}
    {onfocus}
    onclick={onopen}
    onkeydown={onkeys}
    onkeyup={(e) => e.key === ' ' && e.preventDefault()}
  >
    <StatusBadge letter={sideLetter(c, staged)} />
    <span class="text">
      <span class="name">{baseName(c.path)}</span>
      {#if detail}<span class="dir">{detail}</span>{/if}
    </span>
  </button>
  {#if c.kind !== 'conflicted'}<button type="button" class="icon-btn" tabindex="-1" disabled={busy} aria-label={`${staged ? 'Unstage' : 'Stage'} ${c.path}`} title={staged ? 'Unstage (Space)' : 'Stage (Space)'} onclick={ontoggle}><Icon name={staged ? 'minus' : 'plus'} /></button>{/if}
  {#if !staged && c.kind !== 'conflicted'}
    <button type="button" class="icon-btn danger" tabindex="-1" disabled={busy} aria-label={`Discard changes to ${c.path}`} title="Discard (Delete)" onclick={ondiscard}><Icon name="trash" /></button>
  {/if}
  {#if c.kind === 'conflicted'}
    <div class="fix" role="group" aria-label={`Resolve ${c.path}`}>
      <button type="button" class="btn small" disabled={busy} onclick={() => onresolve('ours')}>{labels.ours}</button>
      <button type="button" class="btn small" disabled={busy} onclick={() => onresolve('theirs')}>{labels.theirs}</button>
      <button type="button" class="btn small" disabled={busy} onclick={() => onresolve('mark')}>Mark resolved</button>
    </div>
  {/if}
</li>

<style>
  li {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    border-radius: var(--r-s);
  }
  li:hover {
    background: var(--hover);
  }
  li.picked {
    background: var(--selected);
  }
  .open {
    display: flex;
    flex: 1;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    min-height: var(--row-h);
    padding: var(--s-1);
    border: 0;
    background: none;
    color: var(--fg);
    font: inherit;
    text-align: left;
  }
  /* The name stays whole (it wraps before it would run under the icons); the folder gives way first. */
  .text {
    display: flex;
    flex: 1;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 0 var(--s-1);
    min-width: 0;
  }
  .name {
    max-width: 100%;
    overflow-wrap: anywhere;
  }
  .dir {
    flex: 1 1 8ch;
    min-width: 0;
    color: var(--muted);
    font-size: var(--fs-s);
    white-space: normal;
    overflow-wrap: anywhere;
  }
  .danger {
    color: var(--danger);
  }
  .fix {
    display: flex;
    flex: 1 1 100%;
    flex-wrap: wrap;
    gap: var(--s-1);
    /* Lines up under the name: row padding, the 18px badge, the gap. */
    padding: 0 var(--s-1) var(--s-1) calc(var(--s-1) + 18px + var(--s-1));
  }
  .small {
    min-height: var(--control-h-s);
    padding: 0 var(--s-1);
    font-size: var(--fs-s);
  }
  @media (pointer: coarse) {
    .small {
      min-height: 44px;
    }
  }
</style>
