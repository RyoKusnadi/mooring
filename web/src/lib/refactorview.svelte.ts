// Which editor asked for find references or rename. The dialogs load on first use (see Shell.svelte).
import type { EditorView } from '@codemirror/view'

export interface RefactorRequest {
  kind: 'refs' | 'rename'
  view: EditorView
  projectId: string
  projectPath: string
}

export const refactorView = $state<{ request: RefactorRequest | null }>({ request: null })
