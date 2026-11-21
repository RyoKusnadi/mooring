// Opens a file a chat reply linked to. The path was already cut down to a project-relative one by
// classifyLink; the server checks it again (resolve-then-check) when the file is read.
import { readChunk } from './filesapi'
import { fileActions } from './fileactions'
import { app } from './state.svelte'

export async function openProjectFile(projectId: string, path: string, line?: number): Promise<void> {
  try {
    await readChunk(projectId, path, { limit: 1 })
  } catch (e) {
    app.notice = `Could not open ${path}. ` + (e instanceof Error && e.message ? e.message : 'It is not in this project.')
    return
  }
  // The file opens in the project the chat belongs to.
  if (projectId && app.currentId !== projectId) app.select(projectId)
  fileActions.openFile(path, line)
}
