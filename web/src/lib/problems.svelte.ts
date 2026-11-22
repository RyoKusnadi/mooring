// Diagnostics from every running language server, by project, for the Problems list and the
// status bar count. The language-server chunk (lspclient.ts) writes; the shell reads.
import { countProblems, sortProblems, type Problem } from './problems'

interface ConnProblems {
  projectId: string
  files: Record<string, Problem[]>
}

class Problems {
  /** By language-server connection, then by file path. */
  private byConn = $state<Record<string, ConnProblems>>({})

  /** A server's whole list for one file; an empty list clears the file. */
  set(conn: string, projectId: string, path: string, list: Problem[]) {
    const c = (this.byConn[conn] ??= { projectId, files: {} })
    if (list.length) c.files[path] = list
    else delete c.files[path]
  }

  /** The server stopped or lost its connection: what it said no longer holds. */
  drop(conn: string) {
    delete this.byConn[conn]
  }

  list(projectId: string): Problem[] {
    const all: Problem[] = []
    for (const c of Object.values(this.byConn)) {
      if (c.projectId === projectId) for (const l of Object.values(c.files)) all.push(...l)
    }
    return sortProblems(all)
  }

  counts(projectId: string): { errors: number; warnings: number } {
    return countProblems(this.list(projectId))
  }
}

export const problems = new Problems()
