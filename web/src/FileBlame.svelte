<script lang="ts">
  // The "Who changed this" switch of a file tab. Off until pressed; the first press loads the
  // gutter. The gutter is hidden while the file has unsaved edits, because the lines no longer
  // match what Git knows, and comes back after a save.
  import { onDestroy, untrack } from 'svelte'
  import { actions } from './lib/actions'
  import { getBlame } from './lib/blame'
  import { plainError, FileError } from './lib/filesapi'
  import type { FileDoc } from './lib/filedoc.svelte'
  import { files } from './lib/filestore.svelte'
  import { app } from './lib/state.svelte'

  let { doc, id, path }: { doc: FileDoc; id: string; path: string } = $props()

  let on = $state(false)
  let error = $state('')
  let loading = $state(false)
  let note = $state('')
  let noteSha = $state('')
  let lastReload = untrack(() => doc.reloads)
  const waiting = $derived(on && (doc.dirty || doc.saving))

  const WORDS: Record<string, string> = {
    NOT_TRACKED: 'This file is not in Git yet, so there is nothing to show. Commit it first.',
    NOT_A_REPO: 'This project is not a Git repository.',
    TIMEOUT: 'Git took too long to read this file.',
    TOO_LARGE: 'This file is too large to show who changed each line.',
  }

  function toggle() {
    on = !on
  }
  const openLine = () => noteSha && actions.openCommit('', noteSha, noteSha.slice(0, 7))
  $effect(() => {
    files.blamers[id] = { on, toggle, openLine: noteSha ? openLine : undefined }
    return () => delete files.blamers[id]
  })

  // Fetch when the switch is on and the text is the saved text; again after each save or disk change.
  $effect(() => {
    void doc.reloads
    if (!on || doc.status !== 'ready' || doc.progress || doc.dirty || doc.saving) return
    const stop = new AbortController()
    error = ''
    loading = true
    void (async () => {
      try {
        const [mod, blame] = await Promise.all([import('./lib/blamegutter'), getBlame(app.currentId, '', path, stop.signal)])
        const v = doc.view
        if (stop.signal.aborted || !v) return
        if (!mod.hasBlameGutter(v)) {
          const { StateEffect } = await import('@codemirror/state')
          v.dispatch({ effects: StateEffect.appendConfig.of(mod.blameGutter((sha) => actions.openCommit('', sha, sha.slice(0, 7)), (t, sha) => ((note = t), (noteSha = sha)))) })
        }
        v.dispatch({ effects: mod.setBlame.of(blame) })
      } catch (e) {
        if (stop.signal.aborted) return
        error = (e instanceof FileError && WORDS[e.code]) || plainError(e)
      } finally {
        if (!stop.signal.aborted) loading = false
      }
    })()
    return () => stop.abort()
  })

  // Hidden while the lines may not match, and when the switch is off.
  $effect(() => {
    if (doc.status !== 'ready') return
    // A reload replaces the text, so the names on screen belong to the old one until the new ones arrive.
    const reloaded = doc.reloads !== lastReload
    lastReload = doc.reloads
    const hide = !on || doc.dirty || doc.saving || !!error || reloaded
    if (!hide) return
    const v = doc.view
    if (!v) return
    void import('./lib/blamegutter').then((m) => {
      if (m.hasBlameGutter(v) && v === doc.view) v.dispatch({ effects: m.setBlame.of(null) })
    })
  })

  onDestroy(() => delete files.blamers[id])
</script>

<button type="button" class="btn" aria-pressed={on} title="Show who last changed each line" onclick={toggle}>Blame</button>
{#if on}
  {#if error || waiting || loading}
    <span class="state" role="status">{#if error}{error}{:else if waiting}Blame comes back after you save.{:else}Loading blame…{/if}</span>
  {:else if note}
    <span class="state">{note}</span>
    {#if noteSha}<button type="button" class="btn" aria-label="Open commit {noteSha.slice(0, 7)}: {note}" onclick={openLine}>Open commit {noteSha.slice(0, 7)}</button>{/if}
  {/if}
{/if}

<style>
  .state {
    flex: 1 1 24ch;
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .btn[aria-pressed='true'] {
    background: var(--selected);
  }
</style>
