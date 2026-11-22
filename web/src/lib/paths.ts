// Project-relative paths as the files API uses them: "/"-separated, "" is the project folder.

export const joinPath = (dir: string, name: string) => (dir ? `${dir}/${name}` : name)

export function baseName(path: string): string {
  return path.slice(path.lastIndexOf('/') + 1)
}

export function parentPath(path: string): string {
  const i = path.lastIndexOf('/')
  return i < 0 ? '' : path.slice(0, i)
}

/** True when `path` is `dir` or lies inside it. */
export function isInside(path: string, dir: string): boolean {
  return dir === '' || path === dir || path.startsWith(dir + '/')
}

/** Moves a path that lies inside `from` to the same place under `to`. */
export function rebase(path: string, from: string, to: string): string {
  return path === from ? to : to + path.slice(from.length)
}

/** What is wrong with a new file or folder name, or '' when it is fine. */
export function nameProblem(name: string): string {
  if (!name.trim()) return 'Type a name.'
  if (name.includes('/') || name.includes('\0')) return 'A name cannot contain "/".'
  if (name === '.' || name === '..') return 'That name is not allowed.'
  return ''
}

/** Distinguishes two tabs that would both read "index.ts" by adding the nearest folder. */
export function fileLabels(paths: string[]): string[] {
  const count = new Map<string, number>()
  for (const p of paths) count.set(baseName(p), (count.get(baseName(p)) ?? 0) + 1)
  return paths.map((p) => {
    const b = baseName(p)
    if ((count.get(b) ?? 0) < 2) return b
    const dir = baseName(parentPath(p))
    return dir ? `${dir}/${b}` : b
  })
}

/** A free name for a duplicate: "a copy.txt", then "a copy 2.txt". A leading dot is not an extension. */
export function copyName(name: string, taken: (n: string) => boolean): string {
  const dot = name.lastIndexOf('.')
  const stem = dot > 0 ? name.slice(0, dot) : name
  const ext = dot > 0 ? name.slice(dot) : ''
  for (let n = 1; ; n++) {
    const candidate = `${stem} copy${n > 1 ? ' ' + n : ''}${ext}`
    if (!taken(candidate)) return candidate
  }
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(n < 10 * 1024 ? 1 : 0)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}
