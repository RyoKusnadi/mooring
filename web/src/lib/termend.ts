import type { TermInfo } from './termstore.svelte'

/** The line an ended terminal shows. A dropped SSH connection has no exit code worth showing. */
export function endText(info: Pick<TermInfo, 'ssh' | 'code' | 'lost'> | undefined): string {
  if (info?.ssh && info.lost) return 'The connection to the host ended.'
  return info?.code ? `The shell exited with code ${info.code}.` : 'The shell exited.'
}
