<script lang="ts">
  // Shown in a chat when its account hit a usage limit (offer another account) or needs a sign-in
  // (offer Sign in, for an account that is not the agent's own login).
  import { signInAccount, signInTo } from './lib/accountactions'
  import { explain, limited, resetText } from './lib/accountfmt'
  import { accounts } from './lib/accountstore.svelte'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { actions } from './lib/actions'
  import { app } from './lib/state.svelte'

  let { info, agent }: { info: ChatInfo; agent: string } = $props()

  const account = $derived(accounts.get(info.spec?.account?.id ?? '') ?? accounts.forProject(info.projectId, info.agent))
  const target = $derived(info.switchTo ? accounts.get(info.switchTo) : undefined)
  const hit = $derived(!!account && info.state === 'idle' && limited(account.usage))
  const signable = $derived(signInAccount(info))
  let busy = $state(false)
  let error = $state('')
  /** What the last switch did, kept after the limit clears so the user still sees what to do next. */
  let switched = $state('')

  $effect(() => {
    if (info.state === 'running') switched = ''
  })

  async function run(f: () => Promise<unknown>) {
    busy = true
    error = ''
    try {
      await f()
    } catch (e) {
      error = explain((e as { code?: string }).code ?? '', e instanceof Error ? e.message : '')
    } finally {
      busy = false
    }
  }

  const move = () =>
    run(async () => {
      const to = target
      if (!to) return
      await accounts.setProjectAccount(info.projectId, info.agent, to.id)
      switched = `This project now uses ${to.name}. Send your message again.`
    })
</script>

{#if hit && account}
  <section class="panel" aria-label="Usage limit reached">
    <p><strong>The usage limit of {account.name} was reached.</strong> {agent} stopped this turn; {resetText(account.usage.limitResetsAt)}.</p>
    {#if target}
      <p>{target.name} is signed in and has room. Switching moves every {agent} chat in this project to it, then you send your message again.</p>
      <div class="row"><button type="button" class="btn primary" disabled={busy} onclick={move}>Switch to {target.name}</button></div>
    {:else}
      <p>No other signed-in {agent} account has room. Add one, or sign in to one you have, then come back.</p>
      <div class="row"><button type="button" class="btn" onclick={actions.openAccounts}>Open accounts</button></div>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </section>
{:else if signable}
  <section class="panel" aria-label="Account sign-in needed">
    <p><strong>{signable.name} is not signed in.</strong> Sign in opens a terminal for this account. Finish there, then resume the chat.</p>
    <div class="row">
      <button type="button" class="btn primary" disabled={busy} onclick={() => run(() => signInTo(signable.id))}>Sign in to {signable.name}</button>
      <button type="button" class="btn" disabled={busy} onclick={() => run(() => chats.resume((n, a) => app.run(n, a), info.id))}>Resume chat</button>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </section>
{:else if switched}
  <p class="status" role="status">{switched}</p>
{/if}

<style>
  .panel {
    display: grid;
    justify-items: start;
    gap: var(--s-2);
    padding: var(--s-3) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    border-bottom: 1px solid var(--border);
    background: var(--attention-bg);
    color: var(--fg);
  }
  .panel p,
  .status {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .status {
    padding: var(--s-2) max(var(--s-4), calc((100% - var(--chat-col)) / 2));
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  .error {
    color: var(--danger);
  }
</style>
