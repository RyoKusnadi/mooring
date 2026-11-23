// The file filter: narrows what is already loaded by name. It never fetches a folder.
import type { Entry } from './filesapi'
import { files } from './filestore.svelte'
import { joinPath } from './paths'

class TreeFilter {
  query = $state('')

  get active(): boolean {
    return this.query.trim() !== ''
  }

  private self(e: Entry): boolean {
    return e.name.toLowerCase().includes(this.query.trim().toLowerCase())
  }

  /** A loaded entry below this folder matches. */
  below(project: string, path: string): boolean {
    return !!files.dir(project, path)?.entries.some((e) => this.self(e) || (e.kind === 'dir' && this.below(project, joinPath(path, e.name))))
  }

  /** Whether a row shows: it matches, a loaded entry below it matches, or its folder matched. */
  shows(project: string, dir: string, e: Entry, inherited = false): boolean {
    if (!this.active || inherited) return true
    return this.self(e) || (e.kind === 'dir' && this.below(project, joinPath(dir, e.name)))
  }

  /** A folder is on show as open when something below it matches, even if the user left it closed. */
  forced(project: string, dir: string, e: Entry): boolean {
    return this.active && e.kind === 'dir' && this.below(project, joinPath(dir, e.name))
  }

  /** How many loaded entries match the query, in open and closed folders alike. */
  count(project: string, path = ''): number {
    let n = 0
    for (const e of files.dir(project, path)?.entries ?? []) {
      if (this.self(e)) n++
      if (e.kind === 'dir') n += this.count(project, joinPath(path, e.name))
    }
    return n
  }

  matched(e: Entry): boolean {
    return this.active && this.self(e)
  }
}

export const treeFilter = new TreeFilter()
