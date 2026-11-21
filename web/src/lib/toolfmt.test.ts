import { describe, expect, it } from 'vitest'
import type { ToolServer, Var } from './toolapi'
import { draftProblem, emptyDraft, missingRequired, needs, safeUrl, secretsPayload, splitArgs, statusOf, varsFromNames } from './toolfmt'

const v = (name: string, o: Partial<Var> = {}): Var => ({ name, description: '', required: false, secret: true, set: false, ...o })

describe('secretsPayload', () => {
  it('leaves a saved value out unless replaced or cleared', () => {
    const vars = [v('A', { set: true }), v('B', { set: true }), v('C', { set: true }), v('D')]
    const p = secretsPayload(vars, [], { A: { mode: 'keep', value: '' }, B: { mode: 'replace', value: 'new' }, C: { mode: 'clear', value: 'ignored' }, D: { mode: 'replace', value: 'fresh' } }, {})
    expect(p.env).toEqual({ B: 'new', C: '', D: 'fresh' })
  })

  it('does not send an empty value for a name with nothing saved', () => {
    expect(secretsPayload([v('A')], [], { A: { mode: 'replace', value: '' } }, {}).env).toEqual({})
  })
})

describe('missingRequired', () => {
  it('accepts a saved value and a typed one, and names a missing one', () => {
    const vars = [v('A', { required: true, set: true }), v('B', { required: true })]
    expect(missingRequired(vars, [], { A: { mode: 'keep', value: '' }, B: { mode: 'replace', value: '' } }, {})).toMatch(/Enter a value for B/)
    expect(missingRequired(vars, [], { A: { mode: 'keep', value: '' }, B: { mode: 'replace', value: 'x' } }, {})).toBe('')
    expect(missingRequired(vars, [], { A: { mode: 'clear', value: '' }, B: { mode: 'replace', value: 'x' } }, {})).toMatch(/Enter a value for A/)
  })
})

describe('draftProblem', () => {
  it('asks for what is missing in plain words', () => {
    expect(draftProblem(emptyDraft())).toMatch(/name/)
    expect(draftProblem({ ...emptyDraft(), name: 'x' })).toMatch(/command/)
    expect(draftProblem({ ...emptyDraft('http'), name: 'x', url: 'example.com' })).toMatch(/https:\/\//)
    expect(draftProblem({ ...emptyDraft('http'), name: 'x', url: 'http://10.0.0.5/mcp' })).toMatch(/private/)
    expect(draftProblem({ ...emptyDraft('http'), name: 'x', url: 'http://10.0.0.5/mcp', allowPrivate: true })).toBe('')
    expect(draftProblem({ ...emptyDraft(), name: 'x', command: 'npx', env: [v('bad name')] })).toMatch(/not a valid environment variable name/)
  })
})

describe('statusOf', () => {
  const s = (o: Partial<ToolServer>) => ({ kind: 'stdio', allowed: true, enabled: false, checkedAt: 0, signIn: '', ...o }) as ToolServer
  const texts = (x: ToolServer, f = '') => statusOf(x, f).map((t) => t.text)
  it('names each state', () => {
    expect(texts(s({ allowed: false }))).toEqual(['Not allowed yet'])
    expect(texts(s({}))).toEqual(['Needs a check'])
    expect(texts(s({}), 'boom')).toEqual(['Check failed'])
    expect(texts(s({ checkedAt: 1 }))).toEqual(['Checked, not enabled'])
    expect(texts(s({ enabled: true, kind: 'http', signIn: 'signedIn' }))).toEqual(['Enabled', 'Signed in'])
    expect(texts(s({ kind: 'http', signIn: 'needed' }))).toEqual(['Needs a check', 'Needs sign-in'])
  })
})

describe('small helpers', () => {
  it('splits arguments by line, keeping spaces inside one', () => {
    expect(splitArgs('-y\r\n@scope/pkg\n\nhello world\n')).toEqual(['-y', '@scope/pkg', 'hello world'])
  })
  it('keeps the vars it already had', () => {
    expect(varsFromNames('A B A', [v('B', { set: true })]).map((x) => [x.name, x.set])).toEqual([['A', false], ['B', true]])
  })
  it('says which program is missing', () => {
    expect(needs({ available: false, missing: 'uvx' } as never)).toBe('not installed here')
    expect(needs({ available: true } as never)).toBe('')
  })
  it('links only http and https', () => {
    expect(safeUrl('javascript:alert(1)')).toBe('')
    expect(safeUrl('https://example.com/a')).toBe('https://example.com/a')
    expect(safeUrl('')).toBe('')
  })
})

describe('explain', () => {
  it('says a busy check plainly and passes other messages through', async () => {
    const { explain } = await import('./toolfmt')
    expect(explain('BUSY', 'x')).toMatch(/already running/)
    const wrong = 'The sign-in reply came from a different sign-in site than expected. Start the sign-in again.'
    const moved = "This tool server's sign-in site changed. Sign in again."
    expect(explain('SIGN_IN_FAILED', wrong)).toBe(wrong)
    expect(explain('SIGN_IN_REQUIRED', moved)).toBe(moved)
    expect(explain('SIGN_IN_FAILED', 'other')).toMatch(/did not finish/)
    expect(explain('SIGN_IN_REQUIRED', 'This server asks you to sign in.')).toMatch(/sign in first/)
    expect(explain('BAD_REQUEST', 'The host nope.invalid could not be found.')).toBe('The host nope.invalid could not be found.')
  })
})
