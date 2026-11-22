// The editors bound to a language server, by project and path, so a jump or a rename can reach
// a file that is open, and wait for one that is opening.
import type { EditorView } from '@codemirror/view'

export const views = new Map<string, EditorView>()
export const waiters = new Set<() => void>()
export const viewKey = (projectId: string, path: string) => `${projectId}\n${path}`

export function waitForView(projectId: string, path: string, ms = 5000): Promise<EditorView | null> {
  const k = viewKey(projectId, path)
  return new Promise((resolve) => {
    const done = (v: EditorView | null) => {
      waiters.delete(check)
      clearTimeout(timer)
      resolve(v)
    }
    const check = () => {
      const v = views.get(k)
      if (v) done(v)
    }
    const timer = setTimeout(() => done(null), ms)
    waiters.add(check)
    check()
  })
}

export const liveView = (projectId: string, path: string): EditorView | null => views.get(viewKey(projectId, path)) ?? null
