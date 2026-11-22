<script lang="ts">
  import { onMount } from 'svelte'
  import { app } from './lib/state.svelte'
  import { chats, type Agent } from './lib/chatstore.svelte'
  import { inline } from './lib/chatfmt'
  import { actions } from './lib/actions'
  import { tools } from './lib/toolstore.svelte'
  import { otherAgent } from './lib/secondopinion'
  import ChatImport from './ChatImport.svelte'
  import ChatSeed, { MAX_CONTEXT } from './ChatSeed.svelte'

  let { id }: { id: string } = $props()

  let starting = $state('')
  let checking = $state(false)
  let error = $state('')
  let errorCode = $state('')
  let own = $state(false)
  let branch = $state('')
  let picked = $state<string[]>([])
  let toolsError = $state(false)
  function retryTools() {
    toolsError = false
    tools.load().catch(() => (toolsError = true))
  }
  let list: HTMLElement | undefined = $state()
  const seed = $derived(chats.seeds[id])
  const seedBytes = $derived(seed ? new TextEncoder().encode(seed.text).length : 0)
  // Agents that can start come first; the ones that are not installed follow.
  const sorted = $derived([...chats.agents].sort((a, b) => Number(b.installed) - Number(a.installed)))
  // A chat from a summary starts with the agent of the chat it came from, when that agent is installed.
  // A second opinion starts with another agent instead.
  const earlier = $derived(seed ? chats.list[seed.fromChat]?.agent : undefined)
  const review = $derived(!!seed?.review)
  const same = $derived(seed && !review ? chats.agents.find((a) => a.id === earlier && a.installed) : undefined)
  const first = $derived(review ? otherAgent(chats.agents, earlier)?.id : (same?.id ?? chats.agents.find((a) => a.installed)?.id))

  onMount(() => {
    if (!tools.loaded) retryTools()
    list?.querySelector<HTMLElement>('button:not(:disabled)')?.focus()
  })

  async function start(a: Agent) {
    starting = a.id
    error = ''
    errorCode = ''
    try {
      await chats.start((n, args) => app.run(n, args), app.currentId, id, a.id, own ? { branch: branch.trim() } : undefined, picked.filter((p) => tools.get(p)?.enabled), seed?.text.trim() ? { fromChat: seed.fromChat, context: seed.text.trim(), ...(seed.review ? { withChanges: true } : {}) } : undefined)
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not start the chat.'
      errorCode = (e as { code?: string }).code ?? ''
    } finally {
      starting = ''
    }
  }

  async function check() {
    checking = true
    error = ''
    try {
      await chats.refreshAgents((n, args) => app.run(n, args))
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not check which agents are installed.'
    } finally {
      checking = false
    }
  }
</script>

<div class="picker">
  <h2>Start a chat</h2>
  <p class="help">{review ? 'Pick the agent that gives the second opinion. A different agent from the earlier chat is the main choice.' : 'Pick an agent.'} It starts in <code>{app.current?.path ?? 'the project folder'}</code> and asks first where the agent supports it.</p>
  {#if !chats.known}
    <p class="help" role="status">Loading agents…</p>
  {:else if chats.agents.length === 0}
    <p class="help">The server lists no agents.</p>
  {/if}
  {#if seed}<ChatSeed {id} />{/if}
  {#if same}
    <button type="button" class="btn primary" data-same-agent disabled={!!starting || seedBytes > MAX_CONTEXT} onclick={() => start(same)}>{starting === same.id ? 'Starting…' : `Start with ${same.name}`}</button>
    <p class="help">The same agent as the earlier chat. Pick another below to switch.</p>
  {/if}
  {#if review && chats.known && !first && chats.agents.some((a) => a.installed)}<p class="help">Only the agent of the earlier chat is installed. You can still pick it below.</p>{/if}
  {#if error || chats.startError[id]}
    <p class="error" role="alert">{error || chats.startError[id]}{#if review && (errorCode === 'NO_COMMIT' || errorCode === 'NOT_A_REPO')} A second opinion needs a git folder with at least one commit. Make a commit, or clear the summary box to start without the changes.{/if}</p>
  {/if}
  <ul bind:this={list}>
    {#each sorted as a (a.id)}
      <li class:off={!a.installed}>
        <div class="info">
          <span class="name">{a.name}</span>
          {#if !a.installed}
            <span class="why">Not installed. Mooring looks for the <code>{a.missing}</code> command and did not find it. Install {a.name}, then check again.</span>
          {:else if a.note}<span class="why">{#each inline(a.note) as s, i (i)}{#if s.code}<code>{s.text}</code>{:else}{s.text}{/if}{/each}</span>{/if}
        </div>
        {#if a.installed}
          <button type="button" class="btn" class:primary={a.id === first && !same} data-primary={a.id === first && !same ? '' : undefined} disabled={!!starting || seedBytes > MAX_CONTEXT} onclick={() => start(a)}>
            {starting === a.id ? 'Starting…' : 'Start chat'}<span class="sr"> with {a.name}</span>
          </button>
        {:else}
          <button type="button" class="btn" onclick={actions.newTerminal}>Install {a.name}</button>
        {/if}
      </li>
    {/each}
  </ul>
  {#if !seed}<ChatImport {id} />{/if}
  <details class="opts">
    <summary>Options</summary>
    <div class="tree">
      <label class="check"><input type="checkbox" bind:checked={own} /> Work in its own worktree</label>
      <p class="help" id="tree-help">The agent gets a separate checkout on its own branch, so your files stay as they are. Review and commit its work from Changes.</p>
      {#if own}
        <div class="field">
          <label for="tree-branch">Branch name (optional)</label>
          <input id="tree-branch" class="input mono" type="text" autocomplete="off" spellcheck="false" placeholder="mooring/ followed by a random name" aria-describedby="tree-help" bind:value={branch} />
        </div>
      {/if}
    </div>
    <fieldset class="tools">
      <legend>Tool servers for this chat</legend>
      {#if !tools.loaded}
        <p class="help" role="status">{toolsError ? 'Could not read the tool servers. Check that Mooring is running.' : 'Loading tool servers…'}</p>
        {#if toolsError}<button type="button" class="btn" onclick={retryTools}>Try again</button>{/if}
      {:else if tools.enabled.length === 0}
        <p class="help">No tool server is enabled, so the chat gets none. Tool servers give an agent extra tools, such as a database or a browser.</p>
        <button type="button" class="btn" onclick={() => (tools.dialog = true)}>Set up tool servers</button>
      {:else}
        <p class="help" id="tools-help">Only the ones you tick. Each runs the command you allowed, with the saved values.</p>
        {#each tools.enabled as t (t.id)}
          <label class="check"><input type="checkbox" value={t.id} bind:group={picked} aria-describedby="tools-help" /> <span class="tname">{t.name}</span></label>
        {/each}
      {/if}
    </fieldset>
  </details>
  <button type="button" class="btn ghost" disabled={checking} onclick={check}>{checking ? 'Checking…' : 'Check again'}</button>
</div>

<style>
  .picker {
    display: grid;
    justify-items: start;
    gap: var(--s-3);
    max-width: 560px;
    margin: 0 auto;
    padding: var(--s-6) var(--s-4);
  }
  h2 {
    margin: 0;
    font-size: var(--fs-l);
  }
  p {
    margin: 0;
  }
  ul {
    display: grid;
    width: 100%;
    margin: 0;
    padding: 0;
    gap: var(--s-1);
    list-style: none;
  }
  li {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-s);
    background: var(--surface);
  }
  .info {
    display: grid;
    min-width: 0;
  }
  .name {
    font-weight: 600;
  }
  .why {
    color: var(--muted);
    font-size: var(--fs-s);
    overflow-wrap: anywhere;
  }
  .btn {
    flex: none;
  }
  .help {
    overflow-wrap: anywhere;
  }
  .opts {
    width: 100%;
  }
  .opts summary {
    min-height: var(--control-h);
    font-weight: 600;
    line-height: var(--control-h);
  }
  .opts > :global(*:not(summary)) {
    margin-bottom: var(--s-3);
  }
  .tree {
    display: grid;
    gap: var(--s-2);
    width: 100%;
    max-width: 100%;
  }
  .check {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-weight: 600;
    cursor: pointer;
  }
  .tools {
    display: grid;
    justify-items: start;
    gap: var(--s-2);
    width: 100%;
    max-width: 100%;
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    font-weight: 600;
  }
  .tname {
    overflow-wrap: anywhere;
  }
  .tools .check {
    min-height: var(--control-h);
  }
  .check input {
    width: 16px;
    height: 16px;
    margin: 0;
    accent-color: var(--accent);
  }
</style>
