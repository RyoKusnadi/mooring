// HTTP half of the settings contract.
// Saving goes over the socket; the current values are read here.
import type { Fetcher } from './api'
import { getJSON } from './filesapi'

export interface Settings {
  compactDefault: boolean
  /** Agent id to model id. An agent without an entry has no small-model jobs. */
  smallModel: Record<string, string>
  titleJobs: boolean
  commitJobs: boolean
  jobTokensPerDay: number
}

export interface Model {
  id: string
  name: string
}

/**
 * The model to suggest for small jobs, from what the agent offers: for Claude, the first
 * Haiku (its smallest family). No other agent gets a suggestion. It is only suggested, never chosen.
 */
export function suggestSmall(agent: string, models: Model[]): Model | undefined {
  if (agent !== 'claude') return undefined
  return models.find((m) => /haiku/i.test(m.id) || /haiku/i.test(m.name))
}

export const MIN_JOB_TOKENS = 1000
export const MAX_JOB_TOKENS = 100_000_000

const browserFetch: Fetcher = (input, init) => fetch(input, init)

export const fetchSettings = (f: Fetcher = browserFetch) => getJSON<Settings>('/api/settings', f)

/** The raw text of one saved command output. */
export async function fetchOutput(chatId: string, outputId: string, f: Fetcher = browserFetch): Promise<string> {
  const res = await f(`/api/chats/${encodeURIComponent(chatId)}/outputs/${encodeURIComponent(outputId)}`)
  if (res.status === 404) throw new Error('This output is no longer kept. Mooring keeps the last 50 per chat.')
  if (!res.ok) throw new Error('Could not read the output. Try again.')
  return res.text()
}
