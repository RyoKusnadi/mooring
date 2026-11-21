// HTTP half of the accounts contract.
// Commands go over the socket; the list is read here.
import type { Fetcher } from './api'
import { getJSON } from './filesapi'

export interface UsageWindow {
  name: string
  /** null when the agent did not say. */
  usedPercent: number | null
  /** Milliseconds; 0 when unknown. */
  resetsAt: number
  status: string
}

export interface Usage {
  source: 'agent' | 'unknown'
  windows: UsageWindow[] | null
  limitHit: number
  limitResetsAt: number
  updatedAt: number
}

export type LoginState = 'unknown' | 'signedIn' | 'needed'

export interface Account {
  id: string
  agent: string
  name: string
  dir: string
  managed: boolean
  isDefault: boolean
  login: LoginState
  usage: Usage
}

export interface AccountsReply {
  accounts: Account[]
  /** Project id, then agent, to the account id the project uses. */
  projects: Record<string, Record<string, string>>
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export const fetchAccounts = (f: Fetcher = browserFetch) => getJSON<AccountsReply>('/api/accounts', f)
