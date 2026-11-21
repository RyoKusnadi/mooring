// Files the file tab shows as pictures. The server checks the content as well; this only decides
// whether to ask it. Same list as the raw route in internal/files/raw.go.
const IMAGE = /\.(png|jpe?g|gif|webp|avif|bmp|ico|svg)$/i

export const isImagePath = (path: string): boolean => IMAGE.test(path)

/** The address of a project file served as it is, for an <img>. `version` changes when the file does. */
export function rawUrl(projectId: string, path: string, version: string | number): string {
  return `/api/files/raw?project=${encodeURIComponent(projectId)}&path=${encodeURIComponent(path)}&v=${encodeURIComponent(String(version))}`
}
