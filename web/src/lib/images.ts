// Image attachments of a chat message: what the browser checks before it uploads, and the upload itself.
// The server checks again by what the bytes say. The bytes never travel inside a command.

export const MAX_IMAGE_BYTES = 5 * 1024 * 1024
export const MAX_MESSAGE_ATTACHMENTS = 8
export const IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/gif', 'image/webp']
export const IMAGE_ACCEPT = IMAGE_TYPES.join(',')

type Fetcher = (input: string, init?: RequestInit) => Promise<Response>

export interface UploadedImage {
  id: string
  name: string
  size: number
}

/** "312 KB" or "9.1 MB". */
export function fmtBytes(n: number): string {
  return n >= 1024 * 1024 ? `${(n / (1024 * 1024)).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`
}

export const imageUrl = (chatId: string, imageId: string) => `/api/chats/${encodeURIComponent(chatId)}/images/${encodeURIComponent(imageId)}`

export const notAnImage = (name: string) => `${name} is not an image. Attach PNG, JPEG, GIF or WebP.`
export const tooBig = (name: string, size: number) => `${name} is ${fmtBytes(size)}. Images can be up to 5 MB.`
export const TOO_MANY = `A message takes at most ${MAX_MESSAGE_ATTACHMENTS} attachments. Remove one first.`

export function noImagesNote(agent: string): string {
  return `${agent} does not take images. Start a chat with an agent that does, such as Claude.`
}

/** Splits files into those that can be attached and the first problem. `room` is how many more attachments fit. */
export function checkImages(files: File[], room: number): { ok: File[]; error: string } {
  const ok: File[] = []
  let error = ''
  for (const f of files) {
    if (!IMAGE_TYPES.includes(f.type)) error ||= notAnImage(f.name)
    else if (f.size > MAX_IMAGE_BYTES) error ||= tooBig(f.name, f.size)
    else if (ok.length >= room) {
      error ||= TOO_MANY
      break
    } else ok.push(f)
  }
  return { ok, error }
}

const browserFetch: Fetcher = (input, init) => fetch(input, init)

/** Sends one image to the chat's folder on the server and returns its id. */
export async function uploadImage(chatId: string, file: File, f: Fetcher = browserFetch): Promise<UploadedImage> {
  const res = await f(`/api/chats/${encodeURIComponent(chatId)}/images?name=${encodeURIComponent(file.name)}`, { method: 'POST', body: file })
  if (res.status === 413) throw new Error(tooBig(file.name, file.size))
  if (res.status === 415) throw new Error(notAnImage(file.name))
  if (res.status === 429) throw new Error('This chat holds too many unsent images. Send or remove some first.')
  if (!res.ok) throw new Error(`Could not attach ${file.name}. Try again.`)
  const body = (await res.json()) as UploadedImage
  return { id: body.id, name: body.name, size: body.size }
}
