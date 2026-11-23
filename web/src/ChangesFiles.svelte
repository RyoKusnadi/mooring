<script module lang="ts">
  export type Pick = { path: string; staged: boolean }
</script>

<script lang="ts">
  // The files of the Changes view: staged, then not staged. One row is the tab stop; Up and Down move
  // through every row and show its diff, Space stages or unstages, Delete asks to discard.
  import { tick, untrack } from 'svelte'
  import ChangesRow from './ChangesRow.svelte'
  import DiscardConfirm from './DiscardConfirm.svelte'
  import { survivor } from './lib/focusnext'
  import { pathsOf, type Change } from './lib/gitapi'
  import { sideLabels } from './lib/gitoperation'
  import { git } from './lib/gitstore.svelte'
  import { parentPath } from './lib/paths'

  let { staged, unstaged, sel, onselect, onopen }: { staged: Change[]; unstaged: Change[]; sel: Pick | null; onselect: (p: Pick) => void; onopen: () => void } = $props()

  /** What a discard is about to throw away (the paths as they were when it was asked), and whether the trash is missing. */
  let confirm = $state<{ paths: string[]; label: string; note: string; forever: boolean } | null>(null)
  let asking: { show: () => void } | undefined = $state()
  let root: HTMLElement | undefined = $state()
  let want = $state<string | null>(null)
  // The last row the user was on, and whether focus is inside this list: both survive a row vanishing under focus.
  let last = ''
  let inside = false
  let prevKeys: string[] = []
  const busy = $derived(git.busy !== '')
  const keyOf = (p: Pick) => `${p.staged ? 's' : 'u'}:${p.path}`
  const rows = $derived<Pick[]>([...staged.map((c) => ({ path: c.path, staged: true })), ...unstaged.map((c) => ({ path: c.path, staged: false }))])
  const tabKey = $derived(sel && rows.some((r) => keyOf(r) === keyOf(sel)) ? keyOf(sel) : rows[0] ? keyOf(rows[0]) : '')
  const discardable = $derived(unstaged.filter((c) => c.kind !== 'conflicted'))
  const skipped = $derived(unstaged.length - discardable.length)
  const detail = (c: Change, s: boolean) => (c.origPath && s ? `was ${c.origPath}` : parentPath(c.path))

  const el = (k: string) => [...(root?.querySelectorAll<HTMLElement>('button[data-key]') ?? [])].find((b) => b.dataset.key === k)
  const lost = () => !document.activeElement || document.activeElement === document.body || !document.activeElement.isConnected

  // A stage or unstage reshuffles the rows; focus goes where the user's selection moved to.
  $effect(() => {
    void rows
    const k = want
    if (!k) return
    untrack(() => {
      void tick().then(() => {
        const b = el(k)
        if (!b) return
        b.focus()
        want = null
      })
    })
  })

  // A row can vanish under focus (an agent edit, a discard): focus moves to the next row, else the previous, else the view.
  function recover(before: string[]) {
    const to = survivor(before, rows.map(keyOf), last)
    const row = rows.find((r) => keyOf(r) === to)
    if (!row) return void (root?.closest<HTMLElement>('section')?.focus())
    onselect(row)
    last = to
    el(to)?.focus()
  }
  $effect(() => {
    const now = rows.map(keyOf)
    untrack(() => {
      const before = prevKeys
      prevKeys = now
      if (!inside || !last || want || confirm || now.includes(last)) return
      void tick().then(() => lost() && recover(before))
    })
  })

  async function toggle(p: Pick) {
    const list = p.staged ? staged : unstaged
    const i = list.findIndex((c) => c.path === p.path)
    const c = list[i]
    if (!p.staged && c.kind === 'conflicted') return
    const next = list[i + 1] ?? list[i - 1]
    const focused = root?.contains(document.activeElement)
    const ok = await (p.staged ? git.unstage(pathsOf(c, true)) : git.stage([c.path]))
    if (!ok) return
    const to: Pick = next ? { path: next.path, staged: p.staged } : { path: p.path, staged: !p.staged }
    onselect(to)
    if (focused) want = keyOf(to)
  }

  function keys(e: KeyboardEvent, p: Pick) {
    const i = rows.findIndex((r) => keyOf(r) === keyOf(p))
    const to = e.key === 'ArrowDown' ? i + 1 : e.key === 'ArrowUp' ? i - 1 : e.key === 'Home' ? 0 : e.key === 'End' ? rows.length - 1 : -2
    if (to !== -2) {
      e.preventDefault()
      if (rows[to]) el(keyOf(rows[to]))?.focus()
    } else if (e.key === ' ') {
      e.preventDefault()
      if (!busy) void toggle(p)
    } else if (e.key === 'Delete' || (e.key === 'Backspace' && e.metaKey)) {
      const c = unstaged.find((x) => x.path === p.path)
      if (!p.staged && c && c.kind !== 'conflicted') {
        e.preventDefault()
        askOne(c.path)
      }
    }
  }

  const askOne = (path: string) => void ask([path], `changes to ${path}`, '')

  function askAll() {
    const n = discardable.length
    const parts: string[] = []
    if (staged.length > 0) parts.push(`The ${staged.length} staged ${staged.length === 1 ? 'file stays' : 'files stay'} staged.`)
    if (discardable.some((c) => c.kind === 'untracked')) parts.push('New files are removed too.')
    if (unstaged.some((c) => c.kind === 'conflicted')) parts.push('Conflicted files are skipped.')
    void ask(discardable.map((c) => c.path), n === 1 ? '1 unstaged change' : `all ${n} unstaged changes`, parts.join(' '))
  }

  async function ask(paths: string[], label: string, note: string, forever = false) {
    confirm = { paths, label, note, forever }
    await tick()
    asking?.show()
  }

  async function run() {
    if (!confirm) return
    const { paths, label, note, forever } = confirm
    const before = rows.map(keyOf)
    const r = await git.discard(paths, forever)
    if (r === 'trash') return ask(paths, label, note, true)
    confirm = null
    if (r !== 'done') return
    await tick()
    if (lost() && last) recover(before)
  }

  function stop() {
    confirm = null
    void tick().then(() => (tabKey ? el(tabKey) : undefined)?.focus())
  }

  function track(e: FocusEvent) {
    inside = true
    const k = (e.target as HTMLElement).closest<HTMLElement>('li[data-row]')?.dataset.row
    if (k) last = k
  }
  const leave = (e: FocusEvent) => {
    if (e.relatedTarget && !root?.contains(e.relatedTarget as Node)) inside = false
  }
</script>

{#snippet list(isStaged: boolean, items: Change[])}
  <div class="bar">
    <h3>{isStaged ? 'Staged' : 'Changes'} <span class="count">{items.length}</span></h3>
    <button type="button" class="btn ghost small" disabled={busy || (!isStaged && discardable.length === 0)} title={!isStaged && skipped > 0 ? `${skipped} conflicted ${skipped === 1 ? 'file is' : 'files are'} left out. Resolve with its buttons.` : undefined} onclick={() => (isStaged ? git.unstage(['']) : git.stage(['']))}>{isStaged ? 'Unstage all' : 'Stage all'}</button>
  </div>
  <ul aria-label={isStaged ? 'Staged files' : 'Unstaged files'}>
    {#each items as c (c.path + (isStaged ? ':s' : ':u'))}
      {@const p = { path: c.path, staged: isStaged }}
      <ChangesRow
        {c}
        staged={isStaged}
        picked={!!sel && keyOf(sel) === keyOf(p)}
        key={keyOf(p)}
        tab={keyOf(p) === tabKey}
        {busy}
        detail={detail(c, isStaged)}
        onfocus={() => onselect(p)}
        onopen={() => (onselect(p), onopen())}
        onkeys={(e) => keys(e, p)}
        ontoggle={() => toggle(p)}
        ondiscard={() => askOne(c.path)}
        labels={sideLabels(git.status?.operation)}
        onresolve={(side) => git.resolve(c.path, side)}
      />
    {/each}
  </ul>
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions: focus events only record where the user was -->
<div class="files" bind:this={root} onfocusin={track} onfocusout={leave}>
  {#if staged.length > 0}{@render list(true, staged)}{/if}
  {#if unstaged.length > 0}{@render list(false, unstaged)}{/if}
  {#if confirm}
    <DiscardConfirm bind:this={asking} label={confirm.label} note={confirm.note} forever={confirm.forever} {busy} onrun={run} onkeep={stop} />
  {:else if discardable.length > 0}
    <div class="away">
      <button type="button" class="btn ghost small danger" disabled={busy} onclick={askAll}>Discard all</button>
    </div>
  {/if}
</div>

<style>
  .files {
    padding-bottom: var(--s-2);
  }
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-1);
    padding: var(--s-1) var(--s-2) 0 var(--s-3);
  }
  .bar:not(:first-child) {
    border-top: 1px solid var(--border);
  }
  h3 {
    margin: 0 var(--s-1) 0 0;
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .count {
    color: var(--muted);
    font-weight: var(--fw-regular);
  }
  .small {
    min-height: var(--control-h-s);
    padding: 0 var(--s-1);
    font-size: var(--fs-s);
  }
  .danger {
    color: var(--danger);
  }
  .away {
    display: flex;
    justify-content: flex-end;
    margin-top: var(--s-3);
    padding: var(--s-2) var(--s-2) 0;
    border-top: 1px solid var(--border);
  }
  ul {
    margin: 0;
    padding: 0 var(--s-2) var(--s-1);
    list-style: none;
  }
  @media (pointer: coarse) {
    .small {
      min-height: 44px;
    }
  }
</style>
