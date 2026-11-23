<script lang="ts">
  // On the new-chat tab: list the sessions an agent made in its own program for this project, and
  // bring one in as a chat. The agent starts briefly to answer, so listing can take a few seconds.
  import { onMount } from 'svelte'
  import { app } from './lib/state.svelte'
  import { chats } from './lib/chatstore.svelte'
  import { chatImport as imp, sessionName, sessionTime } from './lib/chatimport.svelte'
  import { relativeTime } from './lib/historyapi'

  let { id }: { id: string } = $props()

  const installed = $derived(chats.agents.filter((a) => a.installed))
  const agent = $derived(installed.some((a) => a.id === imp.agent) ? imp.agent : (installed[0]?.id ?? ''))
  const name = $derived(chats.agents.find((a) => a.id === agent)?.name ?? agent)
  let open = $state(imp.wanted)
  let box: HTMLDetailsElement | undefined = $state()

  onMount(() => {
    imp.reset()
    if (imp.wanted) {
      imp.wanted = false
      box?.querySelector<HTMLElement>('select, button')?.focus()
      box?.scrollIntoView({ block: 'nearest' })
    }
  })

  const find = () => imp.find((n, a) => app.run(n, a), app.currentId, agent)
  const bring = (s: Parameters<typeof imp.bring>[4]) => imp.bring((n, a) => app.run(n, a), app.currentId, id, agent, s)
</script>

{#if installed.length > 0}
  <details class="import" bind:open bind:this={box}>
    <summary>Bring in a chat from an agent's own program</summary>
    <p class="help">Lists the chats {name || 'the agent'} has for <code>{app.current?.path ?? 'this folder'}</code> that are not open in Mooring yet. The agent starts for a moment to answer.</p>
    <div class="row">
      {#if installed.length > 1}
        <label class="sr" for="import-agent">Agent</label>
        <select id="import-agent" class="input" value={agent} disabled={imp.loading} onchange={(e) => ((imp.agent = e.currentTarget.value), imp.reset())}>
          {#each installed as a (a.id)}<option value={a.id}>{a.name}</option>{/each}
        </select>
      {/if}
      <button type="button" class="btn" disabled={imp.loading || !!imp.importing} onclick={find}>{imp.loading ? 'Looking…' : `Find chats from ${name}`}</button>
    </div>
    {#if imp.loading}<p class="help" role="status">Asking {name} for its chats…</p>{/if}
    {#if imp.error}<p class="error" role="alert">{imp.error}</p>{/if}
    {#if imp.sessions && imp.sessions.length === 0}
      <p class="help" role="status">{name} has no chats for this folder that are not already open here.</p>
    {:else if imp.sessions}
      <ul aria-label="Chats from {name}">
        {#each imp.sessions as s (s.sessionId)}
          {@const t = sessionTime(s)}
          <li>
            <span class="what"><span class="title">{sessionName(s)}</span>{#if t !== null}<span class="age">{relativeTime(t)}</span>{/if}</span>
            <button type="button" class="btn" disabled={!!imp.importing} onclick={() => bring(s)}>{imp.importing === s.sessionId ? 'Bringing in…' : 'Bring in'}<span class="sr">: {sessionName(s)}</span></button>
          </li>
        {/each}
      </ul>
    {/if}
  </details>
{/if}

<style>
  .import {
    width: 100%;
  }
  summary {
    min-height: var(--control-h);
    font-weight: var(--fw-semibold);
    cursor: pointer;
  }
  .import > :global(*:not(summary)) {
    margin-top: var(--s-2);
  }
  p {
    margin: 0;
  }
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2);
  }
  ul {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0;
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-l);
    background: var(--surface);
  }
  .what {
    display: grid;
    min-width: 0;
  }
  .title {
    overflow-wrap: anywhere;
  }
  .age {
    color: var(--muted);
    font-size: var(--fs-s);
  }
  .error {
    color: var(--danger);
  }
</style>
