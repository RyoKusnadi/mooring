import { describe, expect, it } from 'vitest'
import { busyReason, canTake, lastReplyId, startNote } from './chatrelay'

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
    expect(busyReason('running')).toBe('replying')
    expect(busyReason('needs_login')).toBe('not signed in')
  })
})

describe('stopped chats', () => {
  it('can take a reply and are not busy', () => {
    expect(canTake('stopped')).toBe(true)
    expect(busyReason('stopped')).toBe('')
    expect(canTake('running')).toBe(false)
    expect(canTake('starting')).toBe(false)
    expect(canTake('needs_login')).toBe(false)
  })
  it('says what Send does', () => {
    expect(startNote({ state: 'idle' })).toBe('')
    expect(startNote({ state: 'stopped', messages: 3, caps: { load: true, resume: false } })).toBe('Starts again when you send')
    expect(startNote({ state: 'stopped', messages: 3, caps: { load: false, resume: true } })).toBe('Starts again when you send')
    expect(startNote({ state: 'stopped', messages: 3, caps: { load: false, resume: false } })).toBe('Starts a new session without the earlier messages')
    expect(startNote({ state: 'stopped', messages: 0, caps: { load: false, resume: false } })).toBe('Starts again when you send')
    expect(startNote({ state: 'stopped', messages: 3, caps: null })).toBe('Starts again when you send')
  })
})
