/** Where focus goes when the focused row is gone: the same row if it is still there, else the next row that is, else the previous one, else nothing. */
export function survivor(before: string[], now: string[], from: string): string {
  if (now.includes(from)) return from
  const i = before.indexOf(from)
  if (i < 0) return now[0] ?? ''
  for (let j = i + 1; j < before.length; j++) if (now.includes(before[j])) return before[j]
  for (let j = i - 1; j >= 0; j--) if (now.includes(before[j])) return before[j]
  return ''
}
