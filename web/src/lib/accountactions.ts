// Account actions that touch the workspace layout.
import type { Account } from './accountapi'
import { accounts } from './accountstore.svelte'
import type { ChatInfo } from './chatstore.svelte'
import { newTab, openTab } from './layout'
import { app } from './state.svelte'

/** Opens the terminal the server made for signing in and focuses its tab. Needs a current project. */
export async function signInTo(accountId: string): Promise<void> {
  const projectId = app.currentId
  if (!projectId) throw new Error('Open a project first. The sign-in terminal opens in it.')
  const id = await accounts.login(accountId, projectId)
  const name = accounts.get(accountId)?.name ?? 'account'
  app.edit((l) => openTab(l, newTab(id, 'terminal', `Sign in: ${name}`)))
}

/** The account of a chat, when its own sign-in is missing and Mooring can open that sign-in (not the agent's own login). */
export function signInAccount(info: ChatInfo): Account | undefined {
  if (info.state !== 'needs_login') return undefined
  const a = accounts.get(info.spec?.account?.id ?? '') ?? accounts.forProject(info.projectId, info.agent)
  return a && !a.id.startsWith('default:') ? a : undefined
}
