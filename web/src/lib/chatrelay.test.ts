import { describe, expect, it } from 'vitest'
import { busyReason, lastReplyId } from './chatrelay'

const rows = [
  { id: 0, kind: 'user', text: 'a' },
  { id: 1, kind: 'agent', text: 'Let me look.' },
  { id: 2, kind: 'tool' },
  { id: 3, kind: 'agent', text: 'Done.' },
  { id: 4, kind: 'tool' },
  { id: 5, kind: 'user', text: 'b' },
  { id: 6, kind: 'agent', text: '  ' },
]
const get = (i: number) => rows[i]

describe('lastReplyId', () => {
  it('picks the last agent reply of the turn', () => expect(lastReplyId(get, 4)).toBe(3))
  it('skips empty text', () => expect(lastReplyId(get, 6)).toBeNull())
  it('stops at the message of the user', () => expect(lastReplyId(get, 5)).toBeNull())
})

describe('busyReason', () => {
  it('is empty for an idle chat', () => expect(busyReason('idle')).toBe(''))
  it('names the state', () => {
    expect(busyReason('running')).toBe('answering now')
    expect(busyReason('needs_login')).toBe('not signed in')
  })
})
