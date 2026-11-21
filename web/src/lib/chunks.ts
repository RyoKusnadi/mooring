/** The list in pieces of at most `size`, in order. The server takes 1000 paths per command. */
export function chunks<T>(list: T[], size: number): T[][] {
  const out: T[][] = []
  for (let i = 0; i < list.length; i += size) out.push(list.slice(i, i + size))
  return out.length ? out : [[]]
}
