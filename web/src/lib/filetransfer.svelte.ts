// What the file tree does with a file beyond opening it: move by drag, duplicate, download,
// upload and copy a path. The line under the tree ("Moved a.txt to src.", with Undo) lives in `note`.
import { fileActions } from './fileactions'
import { downloadUrl, FileError, MAX_UPLOAD, plainError, uploadFile } from './filesapi'
import { files } from './filestore.svelte'
import { baseName, copyName, joinPath, parentPath } from './paths'
import { app } from './state.svelte'
import type { Dropped } from './treedrag'

type Choice = 'replace' | 'keep' | 'cancel'

class Note {
  text = $state('')
  bad = $state(false)
  undo = $state<(() => void) | null>(null)
  /** An upload waiting for the user: the name is taken in that folder. */
  ask = $state<{ name: string; folder: string; answer: (c: Choice) => void } | null>(null)
  private timer: ReturnType<typeof setTimeout> | undefined

  say(text: string, bad = false, undo: (() => void) | null = null) {
    clearTimeout(this.timer)
    this.text = text
    this.bad = bad
    this.undo = undo
    if (!bad) this.timer = setTimeout(() => this.clear(), 12_000)
  }

  /** A job in progress: stays until the next message replaces it. */
  working(text: string) {
    clearTimeout(this.timer)
    this.text = text
    this.bad = false
    this.undo = null
  }

  clear() {
    clearTimeout(this.timer)
    this.text = ''
    this.undo = null
  }

  choose(c: Choice) {
    const a = this.ask
    this.ask = null
    a?.answer(c)
  }
}
export const note = new Note()

/** Set `from` to open the "Move to…" dialog for that row. */
export const mover = $state({ from: '' })

/** One sentence for a failure: what could not be done, then the reason or the next step. */
function failure(what: string, e: unknown): string {
  const why = plainError(e)
  if (/^Could not reach the server/.test(why)) return `Could not ${what} because the server could not be reached. Check that it is running.`
  return `Could not ${what}: ${why.charAt(0).toLowerCase()}${why.slice(1)}`
}

const folderName = (dir: string) => (dir === '' ? (app.current?.name ?? 'the project folder') : baseName(dir))
const taken = (dir: string) => new Set(files.dir(app.currentId, dir)?.entries.map((e) => e.name))
const exists = (e: unknown) => e instanceof FileError && e.code === 'ALREADY_EXISTS'

/** Moves a row into a folder (`''` is the project folder). No confirm: Undo moves it back. */
export async function moveInto(from: string, toDir: string) {
  const name = baseName(from)
  const to = joinPath(toDir, name)
  if (taken(toDir).has(name)) return note.say(`${folderName(toDir)} already has ${name}.`, true)
  try {
    await fileActions.renameEntry(from, to)
  } catch (e) {
    return note.say(exists(e) ? `${folderName(toDir)} already has ${name}.` : failure(`move ${name}`, e), true)
  }
  files.expand(app.currentId, toDir)
  note.say(`Moved ${name} to ${folderName(toDir)}.`, false, () => {
    fileActions.renameEntry(to, from).then(
      () => note.clear(),
      (e: unknown) => note.say(failure(`move ${name} back`, e), true),
    )
  })
}

export async function duplicate(path: string) {
  const dir = parentPath(path)
  const used = taken(dir)
  const name = copyName(baseName(path), (n) => used.has(n))
  try {
    const r = await files.copy(app.currentId, path, joinPath(dir, name))
    const left = r.skippedCount ?? r.skipped.length
    note.say(left ? `Copied ${baseName(path)} as ${name}. ${left} link${left === 1 ? ' was' : 's were'} left out.` : `Copied ${baseName(path)} as ${name}.`)
  } catch (e) {
    note.say(failure(`copy ${baseName(path)}`, e), true)
  }
}

export function download(path: string) {
  const a = document.createElement('a')
  a.href = downloadUrl(app.currentId, path)
  a.download = baseName(path)
  a.click()
}

export async function copyText(text: string, what: string) {
  try {
    await navigator.clipboard.writeText(text)
    note.say(`Copied the ${what}.`)
  } catch {
    note.say(`Could not copy the ${what}. Your browser did not allow it.`, true)
  }
}

export const copyPath = (path: string) => copyText(joinPath(app.current?.path ?? '', path), 'path')

/** Opens the file chooser and uploads the choice into `dir`. */
export function chooseAndUpload(dir: string) {
  const input = document.createElement('input')
  input.type = 'file'
  input.multiple = true
  input.onchange = () => void uploadInto(dir, [...(input.files ?? [])])
  input.click()
}

const answer = (name: string, folder: string) => new Promise<Choice>((answer) => (note.ask = { name, folder, answer }))

/** Uploads one file; a taken name asks Replace, Keep both or Cancel. Returns the stored name, or '' when nothing was stored. */
async function uploadOne(project: string, dir: string, file: File, progress: string): Promise<string> {
  if (file.size > MAX_UPLOAD) {
    note.say(`${file.name} is over 100 MB. Copy large files with a terminal.`, true)
    return ''
  }
  let name = file.name
  let replace = false
  const tried = new Set<string>()
  for (;;) {
    note.working(`Uploading ${progress || name} to ${folderName(dir)}…`)
    try {
      await uploadFile(project, joinPath(dir, name), file, replace)
      return name
    } catch (e) {
      if (!exists(e)) {
        note.say(e instanceof FileError && e.code === 'FILE_TOO_LARGE' ? `${name} is over 100 MB. Copy large files with a terminal.` : failure(`upload ${name}`, e), true)
        return ''
      }
      tried.add(name)
      const choice = await answer(name, folderName(dir))
      if (choice === 'cancel') return ''
      if (choice === 'replace') replace = true
      else {
        const used = taken(dir)
        name = copyName(file.name, (n) => used.has(n) || tried.has(n))
      }
    }
  }
}

export async function uploadInto(dir: string, list: File[]) {
  const project = app.currentId
  if (!list.length) return
  files.expand(project, dir)
  note.clear()
  note.bad = false
  const stored: string[] = []
  for (const [i, f] of list.entries()) {
    const name = await uploadOne(project, dir, f, list.length > 1 ? `${f.name} (${i + 1} of ${list.length})` : '')
    if (name) stored.push(name)
  }
  await files.load(project, dir)
  // A failure keeps its message on screen; a summary would hide it.
  if (stored.length && !note.bad) note.say(stored.length === 1 ? `Uploaded ${stored[0]} to ${folderName(dir)}.` : `Uploaded ${stored.length} files to ${folderName(dir)}.`)
}

/** What a drop on the tree does: move a row, upload files, or say that folders cannot be uploaded. */
export function dropInto(d: Dropped | null, dir: string) {
  if (d?.kind === 'move') void moveInto(d.from, dir)
  else if (d?.kind === 'upload') void uploadInto(dir, d.files)
  else if (d?.kind === 'folders') note.say('Folders cannot be uploaded yet. Drop the files inside it.', true)
}

/** The row menu entries this phase adds, between "New folder here" and "Rename…". */
export function transferItems(path: string, isDir: boolean) {
  return [
    isDir ? { label: 'Upload files…', run: () => chooseAndUpload(path) } : { label: 'Download', run: () => download(path) },
    { label: 'Move to…', run: () => (mover.from = path) },
    { label: 'Duplicate', run: () => void duplicate(path) },
    { label: 'Copy path', run: () => void copyPath(path) },
    { label: 'Copy relative path', run: () => void copyText(path, 'relative path') },
  ]
}
