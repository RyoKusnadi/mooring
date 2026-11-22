<script lang="ts">
  // "Rename": ask for the new name, show what would change, then change it.
  import { onMount, tick, untrack } from 'svelte'
  import SshModal from './SshModal.svelte'
  import type { FileEdits } from './lib/lspedits'
  import type { RefactorRequest } from './lib/refactorview.svelte'
  import { modLabel } from './lib/keys'
  import { nameProblem, renameSummary } from './lib/refsview'

  let { request, onClose }: { request: RefactorRequest; onClose: (focus?: boolean) => void } = $props()

  // One dialog serves one request; a new request mounts a new dialog.
  const { view, projectId, projectPath } = untrack(() => request)
  const word = view.state.wordAt(view.state.selection.main.head)
  const old = word ? view.state.sliceDoc(word.from, word.to) : ''

  let step = $state<'name' | 'preview' | 'done'>('name')
  let next = $state(old)
  let problem = $state('')
  let busy = $state(false)
  let plan = $state<FileEdits[]>([])
  let done = $state<{ changed: string[]; failed: { path: string; reason: string }[] } | { refused: string } | null>(null)
  let input: HTMLInputElement | undefined = $state()
  let open = $state<string[]>([])
  let crash = $state('')

  onMount(() => {
    input?.select()
  })

  // Each step puts focus on its main control; the dialog itself only focuses the first one.
  $effect(() => {
    if (step === 'name') return
    void tick().then(() => document.querySelector<HTMLElement>('dialog[open] [data-primary]')?.focus())
  })

  // The first call loads the language server code; the editor already has it, so this is quick.
  const lsp = () => import('./lib/lspbind')

  async function preview(e: SubmitEvent) {
    e.preventDefault()
    problem = nameProblem(old, next)
    if (problem) return
    busy = true
    try {
      const p = await (await lsp()).planRename(view, projectPath, next.trim())
      if (!p.ok) problem = p.reason
      else if (p.files.length === 0) problem = 'The language server found nothing to change.'
      else {
        plan = p.files
        open = (await lsp()).openPaths(projectId, p.files.map((f) => f.path))
        step = 'preview'
      }
    } catch {
      problem = 'The language server could not rename this.'
    } finally {
      busy = false
    }
  }

  async function apply() {
    busy = true
    try {
      done = await (await lsp()).applyRename(projectId, plan)
    } catch {
      crash = 'The rename stopped part way. Open the files it touches and check them, then try again.'
    } finally {
      busy = false
      step = 'done'
    }
  }

  const back = () => {
    if (busy) return true
    if (step === 'preview') {
      step = 'name'
      return true
    }
    return false
  }
</script>

<SshModal title={step === 'done' ? 'Rename finished' : `Rename ${old}`} onCancel={back} {onClose}>
  {#if step === 'name'}
    <form id="rename-form" onsubmit={preview} novalidate>
      <label for="rename-name">New name</label>
      <input id="rename-name" class="input" data-first bind:this={input} bind:value={next} autocomplete="off" spellcheck="false" aria-invalid={problem ? 'true' : undefined} aria-describedby={problem ? 'rename-problem' : undefined} />
      {#if problem}<p class="error" id="rename-problem" role="alert">{problem}</p>{/if}
    </form>
  {:else if step === 'preview'}
    <p class="lead" role="status" data-first tabindex="-1">{renameSummary(plan)}: {old} becomes {next.trim()}.</p>
    <ul>
      {#each plan as f (f.path)}
        <li><span class="path">{f.path}</span><span class="count">{f.edits.length} {f.edits.length === 1 ? 'place' : 'places'}, {open.includes(f.path) ? 'open in an editor, not saved' : 'saved to disk'}</span></li>
      {/each}
    </ul>
    <p class="muted">Files open in an editor change there and stay unsaved, so undo works. The other files are saved to disk at once, and the editor cannot undo that.</p>
  {:else if crash}
    <p class="error" role="alert" data-first tabindex="-1">{crash}</p>
  {:else if done && 'refused' in done}
    <p class="error" role="alert" data-first tabindex="-1">{done.refused} Nothing was changed.</p>
  {:else if done}
    <p class="lead" role="status" data-first tabindex="-1">
      {#if done.changed.length}Changed {done.changed.length} {done.changed.length === 1 ? 'file' : 'files'}.{:else}No file changed.{/if}
    </p>
    {#if done.changed.length}
      <ul>{#each done.changed as p (p)}<li><span class="path">{p}</span></li>{/each}</ul>
      <p class="muted">Files open in an editor are not saved yet. Save them, or undo with {modLabel}Z.</p>
    {/if}
    {#if done.failed.length}
      <p class="error" role="alert">These files were not changed:</p>
      <ul>{#each done.failed as f (f.path)}<li><span class="path">{f.path}</span><span class="why">{f.reason}</span></li>{/each}</ul>
    {/if}
  {/if}
  {#snippet footer()}
    {#if step === 'name'}
      <button type="button" class="btn" onclick={() => onClose()}>Cancel</button>
      <button type="submit" form="rename-form" class="btn primary" disabled={busy}>{busy ? 'Asking…' : 'Preview'}</button>
    {:else if step === 'preview'}
      <button type="button" class="btn" disabled={busy} onclick={() => onClose()}>Cancel</button>
      <button type="button" class="btn primary" data-primary disabled={busy} onclick={apply}>{busy ? 'Renaming…' : 'Rename'}</button>
    {:else}
      <button type="button" class="btn primary" data-primary onclick={() => onClose()}>Close</button>
    {/if}
  {/snippet}
</SshModal>

<style>
  form {
    display: grid;
    gap: var(--s-2);
  }
  .lead,
  .muted,
  .error {
    margin: 0 0 var(--s-2);
  }
  .muted {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .error {
    color: var(--danger);
    font-size: var(--fs-s);
  }
  ul {
    max-height: 40dvh;
    margin: 0 0 var(--s-2);
    padding: 0;
    overflow-y: auto;
    list-style: none;
  }
  li {
    display: flex;
    flex-wrap: wrap;
    gap: 0 var(--s-3);
    padding: var(--s-1) var(--s-2);
  }
  .path {
    min-width: 0;
    font: var(--fs-s) var(--font-mono);
    overflow-wrap: anywhere;
  }
  .count,
  .why {
    color: var(--muted);
    font-size: var(--fs-s);
  }
</style>
