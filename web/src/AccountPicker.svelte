<script lang="ts">
  // Which account one project uses for one agent. A move restarts the chats of that agent in the project.
  import { explain } from './lib/accountfmt'
  import { accounts } from './lib/accountstore.svelte'
  import { chats } from './lib/chatstore.svelte'

  let { projectId, agent, agentName }: { projectId: string; agent: string; agentName: string } = $props()

  const uid = $props.id()
  const current = $derived(accounts.forProject(projectId, agent))
  const open = $derived(Object.values(chats.list).filter((c) => c.projectId === projectId && c.agent === agent && c.state !== 'stopped').length)
  let busy = $state(false)
  let error = $state('')
  let done = $state('')

  async function pick(id: string) {
    if (id === current?.id) return
    busy = true
    error = done = ''
    try {
      const r = await accounts.setProjectAccount(projectId, agent, id)
      done = r.length === 0 ? 'Saved. No chats had to restart.' : `Saved. ${r.length} ${r.length === 1 ? 'chat restarted' : 'chats restarted'} on this account.`
    } catch (e) {
      error = explain((e as { code?: string }).code ?? '', e instanceof Error ? e.message : '')
    } finally {
      busy = false
    }
  }
</script>

<div class="field">
  <label for="{uid}-pick">{agentName} account for this project</label>
  <span class="select"><select id="{uid}-pick" class="input" disabled={busy} value={current?.id ?? ''} onchange={(e) => pick(e.currentTarget.value)}>
    {#each accounts.of(agent) as a (a.id)}<option value={a.id}>{a.name}{a.isDefault && !a.id.startsWith('default:') ? ' (default)' : ''}</option>{/each}
  </select></span>
  <span class="hint">
    {open === 0
      ? `No ${agentName} chats are open here. New ones use this account.`
      : `Changing it restarts the ${open} open ${agentName} ${open === 1 ? 'chat' : 'chats'} in this project on the new account. A chat that is replying must finish first.`}
  </span>
  {#if error}<span class="error" role="alert">{error}</span>{/if}
  {#if done}<span class="hint" role="status">{done}</span>{/if}
</div>

<style>
  .hint {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .error {
    color: var(--danger);
  }
</style>
