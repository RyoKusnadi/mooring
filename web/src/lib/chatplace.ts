// The folder a chat works in, as the Changes view names it: "" for the project itself, else the worktree folder.
// The folder comes from the chat itself, so it does not wait for the worktree list to load.
import type { ChatInfo } from './chatstore.svelte'

const trim = (p: string) => p.replace(/\/+$/, '')

export function chatPlace(_id: string, info: ChatInfo, projectPath = ''): string {
  const own = trim(info.spec?.folder?.path ?? '')
  return own && own !== trim(projectPath) ? own : ''
}

/** The folder tool lines show paths relative to: the chat's worktree, else the project. */
export function chatRoot(info: ChatInfo | undefined, projectPath: string): string {
  return (info && chatPlace('', info, projectPath)) || projectPath
}
