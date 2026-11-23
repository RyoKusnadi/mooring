<script lang="ts">
  // Replace across files, under the project search: the "Replace with" field, a preview of every
  // changed line, then the change. Loads on first use (see FindDialog.svelte).
  import { onMount, tick } from 'svelte'
  import { modLabel } from './lib/keys'
  import { liveView } from './lib/lspviews'
  import { countPlaces, type FilePlan } from './lib/replace'
  import { applyReplace, planReplace, type ReplacePlan } from './lib/replacerun'
  import { plural, replaceSummary } from './lib/refsview'

  type Done = { changed: string[]; failed: { path: string; reason: string }[] } | { refused: string }
  let {
    project,
    query,
    regex,
    matchCase,
    paths,
    phase,
    stage = $bindable(),
    busy = $bindable(),
    onApplied,
  }: { project: string; query: string; regex: boolean; matchCase: boolean; paths: string[]; phase: string; stage: 'edit' | 'preview' | 'done'; busy: boolean; onApplied: () => void } = $props()

  const SHOWN_LINES = 40
  let withText = $state('')
  let problem = $state('')
  let reading = $state('')
  let announce = $state('')
  // What the plan was made for; the search above is frozen after the preview, and the heading shows this.
  let planned = $state({ query: '', withText: '' })
  let plan = $state<ReplacePlan | null>(null)
  let done = $state<Done | null>(null)
  let crash = $state('')
  let root: HTMLElement | undefined = $state()

  onMount(() => document.getElementById('find-replace')?.focus())

  // Each step puts focus on its main control.
  $effect(() => {
    if (stage === 'edit') return
    void tick().then(() => root?.querySelector<HTMLElement>('[data-primary]')?.focus())
  })

  const isOpen = (path: string) => !!liveView(project, path)
  const clip = (t: string) => (t.length > 160 ? t.slice(0, 160) + '…' : t)

  async function preview(e: SubmitEvent) {
    e.preventDefault()
    problem = ''
    if (!query) problem = 'Type the text to find above first.'
    else if (phase === 'searching') problem = 'The search is still running. Try again in a moment.'
    else if (phase === 'cut') problem = 'The search stopped early, so some matches are not listed. Narrow it with Files, then preview.'
    else if (phase !== 'done' || paths.length === 0) problem = 'Nothing was found to replace.'
    if (problem) return
    busy = true
    try {
      reading = ''
      const p = await planReplace(project, paths, { q: query, regex, matchCase }, withText, (n, total) => (reading = `${n} of ${total}`))
      if ('error' in p) problem = p.error
      else if (p.plans.length === 0) problem = p.skipped.length ? 'No match could be read in the listed files.' : 'Nothing to change: the files no longer match.'
      else {
        plan = p
        planned = { query, withText }
        announce = `${replaceSummary(p.plans.length, countPlaces(p.plans))}.`
        stage = 'preview'
      }
    } catch {
      problem = 'The files could not be read. Try again.'
    } finally {
      busy = false
    }
  }

  async function apply() {
    if (!plan) return
    busy = true
    crash = ''
    try {
      done = await applyReplace(project, plan.plans)
    } catch {
      crash = 'The replace stopped part way. Check the files it touches, then search again.'
    } finally {
      busy = false
      stage = 'done'
      announce = crash || (done && 'refused' in done ? done.refused : done ? `Changed ${plural(done.changed.length, 'file')}.` : '')
      onApplied()
    }
  }

  const finish = () => {
    plan = null
    done = null
    crash = ''
    stage = 'edit'
    void tick().then(() => document.getElementById('find-replace')?.focus())
  }
  const back = () => {
    stage = 'edit'
    void tick().then(() => document.getElementById('find-replace')?.focus())
  }

  // Esc steps back a stage instead of closing the whole panel, and does nothing while writing.
  function keydown(e: KeyboardEvent) {
    if (e.key !== 'Escape' || stage === 'edit') return
    e.preventDefault()
    e.stopPropagation()
    if (busy) return
    if (stage === 'preview') back()
    else finish()
  }
  const places = (p: FilePlan) => `${plural(p.edits.length, 'place')}, ${isOpen(p.path) ? 'open in an editor, not saved' : 'saved to disk'}`
</script>

<!-- svelte-ignore a11y_no_static_element_interactions: Escape goes back one step -->
<div class="rep" bind:this={root} onkeydown={keydown}>
  <p class="sr" role="status">{announce}</p>
  {#if stage === 'edit'}
    <form onsubmit={preview} novalidate>
      <label for="find-replace">Replace with</label>
      <input id="find-replace" class="input mono" type="text" autocomplete="off" spellcheck="false" bind:value={withText} aria-describedby="find-replace-help" />
      <p id="find-replace-help" class="muted">{regex ? 'Regex is on: $1 and $<name> use the groups.' : 'Leave it empty to delete the matches.'}</p>
      {#if problem}<p class="error" role="alert">{problem}</p>{/if}
      <div class="acts"><button type="submit" class="btn primary" disabled={busy}>{busy ? `Reading files… ${reading}` : 'Preview replace'}</button></div>
    </form>
  {:else if stage === 'preview' && plan}
    <p class="lead">{replaceSummary(plan.plans.length, countPlaces(plan.plans))}: {planned.query} becomes {planned.withText || 'nothing'}.</p>
    {#if plan.skipped.length}<p class="error">Not read, so not changed: {plan.skipped.join(', ')}</p>{/if}
    <div class="files">
      {#each plan.plans as p (p.path)}
        <section aria-label={p.path}>
          <h3><span class="path">{p.path}</span><span class="muted">{places(p)}</span></h3>
          {#each p.lines.slice(0, SHOWN_LINES) as l (l.line)}
            <div class="chg">
              <span class="ln">{l.line}</span>
              <span class="before"><span class="sr">Before: </span><span aria-hidden="true">− </span>{clip(l.before)}</span>
              <span class="after"><span class="sr">After: </span><span aria-hidden="true">+ </span>{clip(l.after)}</span>
            </div>
          {/each}
          {#if p.lines.length > SHOWN_LINES}<p class="muted">and {p.lines.length - SHOWN_LINES} more lines</p>{/if}
        </section>
      {/each}
    </div>
    <p class="muted">Files open in an editor change there and stay unsaved, so undo ({modLabel}Z) works. The other files are saved to disk at once, and the editor cannot undo that.</p>
    <div class="acts">
      <button type="button" class="btn" disabled={busy} onclick={back}>Back</button>
      <button type="button" class="btn primary" data-primary disabled={busy} onclick={apply}>{busy ? 'Replacing…' : 'Replace'}</button>
    </div>
  {:else}
    {#if crash}
      <p class="error" role="alert">{crash}</p>
    {:else if done && 'refused' in done}
      <p class="error" role="alert">{done.refused} Nothing was changed.</p>
    {:else if done}
      <p class="lead">{done.changed.length ? `Changed ${plural(done.changed.length, 'file')}.` : 'No file changed.'}</p>
      {#if done.changed.length}
        <ul>{#each done.changed as p (p)}<li class="path">{p}</li>{/each}</ul>
        <p class="muted">Files open in an editor are not saved yet. Save them, or undo with {modLabel}Z.</p>
      {/if}
      {#if done.failed.length}
        <p class="error" role="alert">These files were not changed:</p>
        <ul>{#each done.failed as f (f.path)}<li><span class="path">{f.path}</span> <span class="muted">{f.reason}</span></li>{/each}</ul>
      {/if}
    {/if}
    <div class="acts"><button type="button" class="btn primary" data-primary onclick={finish}>Done</button></div>
  {/if}
</div>

<style>
  .rep {
    display: grid;
    gap: var(--s-2);
    min-height: 0;
    padding: var(--s-2) var(--s-3) var(--s-3);
    overflow-y: auto;
  }
  form {
    display: grid;
    gap: var(--s-2);
  }
  p,
  ul {
    margin: 0;
  }
  ul {
    padding: 0;
    list-style: none;
  }
  .lead {
    font-weight: var(--fw-semibold);
  }
  .muted {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-s);
  }
  .acts {
    display: flex;
    gap: var(--s-2);
    justify-content: flex-end;
  }
  .files {
    max-height: 45dvh;
    overflow-y: auto;
  }
  h3 {
    display: flex;
    flex-wrap: wrap;
    gap: 0 var(--s-2);
    margin: var(--s-2) 0 var(--s-1);
    font-size: var(--fs-s);
    font-weight: var(--fw-semibold);
  }
  .path {
    min-width: 0;
    font: var(--fs-s) var(--font-mono);
    overflow-wrap: anywhere;
  }
  h3 .muted {
    font-weight: var(--fw-regular);
  }
  .chg {
    display: grid;
    grid-template-columns: 4ch minmax(0, 1fr);
    margin-bottom: var(--s-1);
    font: var(--fs-s) var(--font-mono);
  }
  .ln {
    grid-row: span 2;
    padding-right: var(--s-1);
    color: var(--muted);
    text-align: right;
  }
  .before,
  .after {
    min-width: 0;
    padding: 0 var(--s-1);
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }
  .before {
    background: var(--diff-del);
  }
  .after {
    background: var(--diff-add);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
