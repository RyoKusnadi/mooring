// The address of one side of an image a diff shows: GET /api/git/image, with the same place and
// commit or staged choice the diff itself was read with.
import { qs } from './filesapi'

export type ImageSide = 'old' | 'new'

/** `commit` set: the commit's side, the path relative to the repository top. Else `staged` picks index against HEAD or working file against index. */
export function gitImageUrl(project: string, folder: string, path: string, side: ImageSide, opts: { staged: boolean; commit?: string }): string {
  const where = opts.commit ? { commit: opts.commit } : { staged: opts.staged ? 1 : 0 }
  return `/api/git/image?${qs({ project, folder, path, side, ...where })}`
}
