<script lang="ts">
  // One row per installed agent: a select for its small model. Models come from the agent itself,
  // which means starting it, so they load on a button press and not when the dialog opens.
  import { chats } from './lib/chatstore.svelte'
  import { suggestSmall, type Settings } from './lib/settingsapi'
  import { settings } from './lib/settingsstore.svelte'
  import { app } from './lib/state.svelte'

  let { save }: { save: (change: Partial<Settings>) => Promise<void> } = $props()

  let busy = $state('')
  let problem = $state<Record<string, string>>({})
  const uid = $props.id()

  const agents = $derived(chats.agents.filter((a) => a.installed))

  async function find(agent: string) {
    if (!app.current) return
    busy = agent
    problem[agent] = ''
    try {
      await settings.loadModels(agent, app.current.id)
    } catch (e) {
      problem[agent] = e instanceof Error ? e.message : 'Could not start the agent.'
    } finally {
      busy = ''
    }
  }

  function pick(agent: string, id: string) {
    const next = { ...settings.value.smallModel }
    if (id) next[agent] = id
    else delete next[agent]
    void save({ smallModel: next })
  }

  /** Models to show: the ones found, plus the saved one until the agent has been asked. */
  function options(agent: string) {
    const found = settings.models[agent]
    const now = settings.value.smallModel[agent]
    if (!found) return now ? [{ id: now, name: now }] : []
    return found
  }

  /** The agent was asked and no longer lists the saved model. */
  function gone(agent: string) {
    const found = settings.models[agent]
    const now = settings.value.smallModel[agent]
    return !!now && found !== undefined && found.length > 0 && !found.some((m) => m.id === now)
  }

  function suggested(agent: string) {
    return suggestSmall(agent, settings.models[agent] ?? [])
  }
</script>

{#if !app.current}
  <p class="help">Open a project to look up models. An agent is started in the project folder to ask for them.</p>
{/if}
<ul>
  {#each agents as a (a.id)}
    {@const none = settings.models[a.id] !== undefined && settings.models[a.id].length === 0}
    {@const hint = suggested(a.id)}
    <li>
      {#if gone(a.id)}<p class="error" role="alert">{a.name} no longer offers <code>{settings.value.smallModel[a.id]}</code>. Small jobs for {a.name} are off until you pick another model.</p>{/if}
      <label for="{uid}-{a.id}">{a.name}</label>
      <select id="{uid}-{a.id}" class="input" disabled={none || (options(a.id).length === 0)} value={gone(a.id) ? '' : (settings.value.smallModel[a.id] ?? '')} onchange={(e) => pick(a.id, e.currentTarget.value)}>
        <option value="">{none ? 'No model option' : 'None set'}</option>
        {#each options(a.id) as m (m.id)}<option value={m.id}>{m.name}{hint?.id === m.id ? ' (suggested)' : ''}</option>{/each}
      </select>
      <button type="button" class="btn" disabled={!app.current || busy !== ''} onclick={() => find(a.id)}>{busy === a.id ? 'Asking…' : 'Find models'}</button>
      {#if hint}<p class="help">{hint.name} is suggested for small jobs: it is the smallest {a.name} model on offer.</p>{/if}
      {#if none}<p class="help">{a.name} has no model option, so it cannot do small jobs.</p>{/if}
      {#if problem[a.id]}<p class="error" role="alert">{problem[a.id]}</p>{/if}
    </li>
  {/each}
</ul>

<style>
  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: var(--s-3);
  }
  li {
    display: grid;
    grid-template-columns: 8rem minmax(0, 1fr) auto;
    gap: var(--s-2);
    align-items: center;
  }
  li p {
    grid-column: 2 / -1;
    margin: 0;
  }
  @media (max-width: 520px) {
    li {
      grid-template-columns: minmax(0, 1fr) auto;
    }
    label {
      grid-column: 1 / -1;
    }
    li p {
      grid-column: 1 / -1;
    }
  }
</style>
