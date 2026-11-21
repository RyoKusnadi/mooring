import { beforeEach, describe, expect, it } from 'vitest'
import { defaults, settings } from './settingsstore.svelte'

let calls: { name: string; args: unknown }[]
beforeEach(() => {
  calls = []
  settings.value = { ...defaults }
  settings.models = {}
  settings.attach(((name: string, args: unknown) => {
    calls.push({ name, args })
    if (name === 'agent.models') return Promise.resolve({ models: null })
    return Promise.resolve({ ...defaults, ...(args as { settings: object }).settings })
  }) as never)
})

describe('settings store', () => {
  it('takes the whole settings from an event', () => {
    settings.apply('settings.changed', { ...defaults, commitJobs: true, smallModel: { gemini: 'fast' } })
    expect(settings.value.commitJobs).toBe(true)
    expect(settings.commitReady).toBe(true)
  })

  it('is not ready for commit jobs without a small model', () => {
    settings.apply('settings.changed', { ...defaults, commitJobs: true })
    expect(settings.commitReady).toBe(false)
  })

  it('saves only the named keys and keeps the answer', async () => {
    await settings.save({ titleJobs: true })
    expect(calls[0]).toEqual({ name: 'settings.save', args: { settings: { titleJobs: true } } })
    expect(settings.value.titleJobs).toBe(true)
  })

  it('keeps an agent with no model option as an empty list', async () => {
    expect(await settings.loadModels('claude', 'p1')).toEqual([])
    expect(settings.models.claude).toEqual([])
    expect(calls[0].args).toEqual({ agent: 'claude', projectId: 'p1' })
  })

  it('ignores other events', () => {
    settings.apply('account.changed', {})
    expect(settings.value).toEqual(defaults)
  })
})
