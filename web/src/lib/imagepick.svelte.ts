// Adding images to a message: the file button, paste and drop. It checks, uploads, and puts the
// result in the chat's attachments. The composer owns one of these per chat.
import { attach } from './attachstore.svelte'
import { checkImages, MAX_MESSAGE_ATTACHMENTS, uploadImage, type UploadedImage } from './images'

interface Hooks {
  id: () => string
  /** Whether the chat's agent takes images. */
  takes: () => boolean
  /** Called when files were added, to put the cursor back in the box. */
  done: () => void
}

const hasFiles = (e: DragEvent) => [...(e.dataTransfer?.types ?? [])].includes('Files')

const seen = $state<Record<string, { error: string; refused: boolean }>>({})

export class ImagePicker {
  /** One line under the box: the first problem. The files that were fine are still attached. Kept per chat, so it survives the box being drawn again. */
  get error() {
    return seen[this.h.id()]?.error ?? ''
  }
  set error(v: string) {
    ;(seen[this.h.id()] ??= { error: '', refused: false }).error = v
  }
  /** Set when a paste or drop was refused because the agent takes no images: the note then speaks up. */
  get refused() {
    return seen[this.h.id()]?.refused ?? false
  }
  set refused(v: boolean) {
    ;(seen[this.h.id()] ??= { error: '', refused: false }).refused = v
  }
  drag = $state(false)
  /** Uploads in flight. */
  busy = $state(0)
  private depth = 0

  constructor(private h: Hooks) {}

  /** Drops the error line and the refused note: the chip that caused them is gone, or the message went. */
  clear() {
    this.error = ''
    this.refused = false
  }

  async add(files: File[]) {
    if (!this.h.takes()) {
      this.refused = true
      return
    }
    const id = this.h.id()
    this.error = ''
    this.refused = false
    const { ok, error } = checkImages(files, MAX_MESSAGE_ATTACHMENTS - attach.list(id).length - this.busy)
    this.error = error
    this.busy += ok.length
    const done = await Promise.allSettled(ok.map((f) => uploadImage(id, f)))
    this.busy -= ok.length
    for (const r of done) {
      if (r.status === 'fulfilled') this.put(id, r.value)
      else this.error ||= r.reason instanceof Error ? r.reason.message : 'Could not attach the image. Try again.'
    }
    this.h.done()
  }

  private put(id: string, u: UploadedImage) {
    const err = attach.add(id, { kind: 'image', imageId: u.id, name: u.name, size: u.size })
    if (err) this.error ||= err
  }

  paste = (e: ClipboardEvent) => {
    const files = [...(e.clipboardData?.files ?? [])].filter((f) => f.type.startsWith('image/'))
    if (!files.length) return
    e.preventDefault()
    void this.add(files)
  }

  dragEnter = (e: DragEvent) => {
    if (!hasFiles(e)) return
    e.preventDefault()
    this.depth++
    if (this.h.takes()) this.drag = true
  }

  dragOver = (e: DragEvent) => {
    if (!hasFiles(e)) return
    e.preventDefault()
    if (e.dataTransfer) e.dataTransfer.dropEffect = this.h.takes() ? 'copy' : 'none'
  }

  dragLeave = () => {
    this.depth = Math.max(0, this.depth - 1)
    if (!this.depth) this.drag = false
  }

  drop = (e: DragEvent) => {
    if (!hasFiles(e)) return
    e.preventDefault()
    this.depth = 0
    this.drag = false
    void this.add([...(e.dataTransfer?.files ?? [])])
  }
}
