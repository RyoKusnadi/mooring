// Files the file tab can show drawn instead of as text, and the choice made per tab.
export type PreviewKind = 'markdown' | 'csv' | 'html'

export function previewKind(path: string): PreviewKind | null {
  if (/\.(md|markdown|mdx)$/i.test(path)) return 'markdown'
  if (/\.(csv|tsv)$/i.test(path)) return 'csv'
  if (/\.html?$/i.test(path)) return 'html'
  return null
}

const chosen = new Map<string, boolean>()
/** Whether a tab was last left on Preview. Source is the default. */
export const wasPreview = (tabId: string): boolean => chosen.get(tabId) ?? false
export const rememberPreview = (tabId: string, on: boolean): void => void (on ? chosen.set(tabId, true) : chosen.delete(tabId))
