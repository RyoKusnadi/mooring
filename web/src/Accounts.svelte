<script lang="ts">
  // The "Accounts and usage" dialog: per agent that can have more than one login, the accounts
  // with their sign-in state and usage, the one this project uses, and a way to add another.
  import { onMount, tick } from 'svelte'
  import AccountAdd from './AccountAdd.svelte'
  import AccountPicker from './AccountPicker.svelte'
  import AccountRow from './AccountRow.svelte'
  import SshModal from './SshModal.svelte'
  import { sortAccounts } from './lib/accountfmt'
  import { accounts } from './lib/accountstore.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { app } from './lib/state.svelte'

  let { onClose }: { onClose: () => void } = $props()

  let adding = $state('')
  let error = $state('')
  let box: HTMLElement | undefined = $state()

  onMount(() => {
    void accounts.load().catch(() => (error = 'Could not read the accounts. Check that Mooring is running.'))
  })

  const nameOf = (agent: string) => chats.agents.find((a) => a.id === agent)?.name ?? agent
  const refocus = () => void tick().then(() => box?.querySelector<HTMLElement>('[data-first]')?.focus())

  function onCancel(): boolean {
    const keep = box?.querySelector<HTMLButtonElement>('[data-keep]')
    if (keep) return keep.click(), true
    return false
  }
</script>

<SshModal title="Accounts and usage" {onClose} wide top {onCancel}>
  <div bind:this={box}>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    {#if !accounts.loaded && !error}
      <p class="muted" role="status">Loading…</p>
    {:else if accounts.agents.length === 0}
      <p class="muted">None of the installed agents can use more than one login yet. Each uses its own sign-in.</p>
    {:else}
      <p class="muted lead">An account is one login of an agent, kept in its own settings folder. Mooring never reads or stores the login. Usage comes from what the agent reports while you chat.</p>
      {#each accounts.agents as agent, i (agent)}
        <section aria-label="{nameOf(agent)} accounts">
          <div class="head">
            <h3>{nameOf(agent)}</h3>
            {#if adding !== agent}
              <button type="button" class="btn" data-first={i === 0 ? '' : undefined} onclick={() => (adding = agent)}>Add {nameOf(agent)} account</button>
            {/if}
          </div>
          {#if adding === agent}
            <AccountAdd {agent} agentName={nameOf(agent)} onDone={() => ((adding = ''), refocus())} />
          {/if}
          {#if app.current}
            <AccountPicker projectId={app.current.id} {agent} agentName={nameOf(agent)} />
          {:else}
            <p class="muted">Open a project to choose its {nameOf(agent)} account.</p>
          {/if}
          <ul>
            {#each sortAccounts(accounts.of(agent)) as a (a.id)}
              <AccountRow account={a} onGone={refocus} onSignIn={onClose} />
            {/each}
          </ul>
        </section>
      {/each}
    {/if}
  </div>
</SshModal>

<style>
  .muted {
    margin: 0;
    color: var(--muted);
  }
  .lead {
    margin-bottom: var(--s-3);
  }
  section {
    margin-bottom: var(--s-4);
  }
  .head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    margin-bottom: var(--s-2);
  }
  h3 {
    margin: 0;
    font-size: var(--fs-m);
  }
  ul {
    margin: var(--s-3) 0 0;
    padding: 0;
    list-style: none;
    border-top: 1px solid var(--border);
  }
</style>
