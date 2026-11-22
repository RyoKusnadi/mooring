// The usage line of the account a chat runs on, for its header and menu.
import { usageLine } from './accountfmt'
import { accounts } from './accountstore.svelte'
import type { ChatInfo } from './chatstore.svelte'

export function chatUsage(info: ChatInfo) {
  const account = accounts.get(info.spec?.account?.id ?? '') ?? accounts.forProject(info.projectId, info.agent)
  const line = account ? usageLine(account.usage) : null
  return line && account ? { ...line, name: account.name } : null
}
