<script lang="ts">
  // The "References" and "Rename" buttons in an editor's bar, and their keys (Shift+F12, F2) in the
  // editor. Drawn only while the file has a ready language server; the palette uses the same two commands.
  import { onMount } from 'svelte'
  import type { EditorView } from '@codemirror/view'
  import { files } from './lib/filestore.svelte'
  import { refactorView } from './lib/refactorview.svelte'
  import { app } from './lib/state.svelte'
  import { showKey } from './lib/keys'

  let { id, view, writable }: { id: string; view: EditorView; writable: boolean } = $props()

  async function ask(kind: 'refs' | 'rename') {
    const project = app.projects.find((p) => p.id === app.currentId)
    if (!project) return
    if (kind === 'rename' && !writable) {
      app.notice = 'This file is read-only, so it is not renamed.'
      return
    }
    if (kind === 'rename') {
      const t = (await import('./lib/lspbind')).renameTarget(view)
      if ('error' in t) {
        app.notice = t.error
        return
      }
    }
    refactorView.request = { kind, view, projectId: project.id, projectPath: project.path }
  }

  onMount(() => {
    const run = { refs: () => ask('refs'), rename: () => ask('rename') }
    files.refactors[id] = run
    const key = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey) return
      const refs = e.key === 'F12' && e.shiftKey
      if (!refs && !(e.key === 'F2' && !e.shiftKey)) return
      e.preventDefault()
      if (refs) run.refs()
      else run.rename()
    }
    view.dom.addEventListener('keydown', key)
    return () => {
      view.dom.removeEventListener('keydown', key)
      delete files.refactors[id]
    }
  })
</script>

<button type="button" class="btn" title="Find references ({showKey('Shift+F12')})" onclick={() => files.refactors[id]?.refs()}>References</button>
<button type="button" class="btn" disabled={!writable} title="Rename the name under the cursor (F2)" onclick={() => files.refactors[id]?.rename()}>Rename</button>
