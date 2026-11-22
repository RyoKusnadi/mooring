// The last turn of a chat: what the agent changed between two checkpoints (git.lastTurn, git.turnDiff).
import type { Diff } from './gitapi'

type Run = <T>(name: string, args: unknown) => Promise<T>

export interface TurnInfo {
  start: string
  end: string
  /** When the turn ended, ms since the epoch. */
  at: number
  chatTitle?: string
}
export interface TurnFile {
  path: string
  origPath?: string
  kind: 'added' | 'modified' | 'deleted' | 'renamed' | 'typechange'
  added: number
  removed: number
}
export interface LastTurn {
  turn: TurnInfo | null
  files: TurnFile[]
}

export const getLastTurn = (run: Run, projectId: string, chatId: string) => run<LastTurn>('git.lastTurn', { projectId, chatId })

export const getTurnDiff = (run: Run, projectId: string, chatId: string, path: string, origPath?: string) =>
  run<Diff>('git.turnDiff', { projectId, chatId, path, ...(origPath ? { origPath } : {}) })

const WORDS: Record<TurnFile['kind'], string> = { added: 'Added', modified: 'Changed', deleted: 'Deleted', renamed: 'Renamed', typechange: 'Type changed' }
export const kindWord = (k: TurnFile['kind']) => WORDS[k] ?? k

/** The empty state, said plainly, with the reason when checkpoints are off. */
export function noTurnText(chat: string, off: boolean): string {
  return off ? `Checkpoints are off for this project, so ${chat} has no last turn to show. Turn them on in Settings, under Token saving.` : `${chat} has not finished a turn with a checkpoint yet.`
}
