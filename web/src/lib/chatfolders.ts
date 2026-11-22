// What a chat's info says about the agent's own folders and copies.
// A chat in its own worktree whose agent takes extra folders can also read the project folder:
// the server sends it on every start. Nothing in the chat info says so directly.
import type { ChatInfo } from './chatstore.svelte'

export const readsProject = (info: Pick<ChatInfo, 'spec' | 'caps'>): boolean => !!info.spec?.folder?.worktree && info.caps?.additionalDirectories === true

/** The question before a chat is deleted. When the agent can delete its own copy, the question says it goes too.
 *  A chat brought in from the agent's own program keeps the agent's copy (the server does not ask). */
export const deleteQuestion = (info: Pick<ChatInfo, 'caps' | 'spec'> | undefined, agent: string): string =>
  info?.caps?.delete === true && !info.spec?.agentMade ? `Delete this chat? Mooring removes it and asks ${agent} to delete its own copy.` : 'Delete this chat?'
