<script lang="ts">
  // The health check: start the server as a chat would, list its tools, stop it. Then offer Enable.
  import { onMount, tick } from 'svelte'
  import { CommandError } from './lib/events'
  import { explain, isRemote, listedTime } from './lib/toolfmt'
  import { tools } from './lib/toolstore.svelte'

  let { id, auto = false, onDone, onSignIn }: { id: string; auto?: boolean; onDone: () => void; onSignIn: (id: string) => void } = $props()

  type Phase = 'running' | 'ok' | 'failed'
  let phase = $state<Phase>('running')
  let reason = $state('')
  let error = $state('')
  let secs = $state(0)
  let cached = $state(false)
  let box: HTMLDivElement | undefined = $state()
  let busy = $state(false)
  const s = $derived(tools.get(id))

  async function run(fresh = false) {
    phase = 'running'
    reason = ''
    secs = 0
    const t = setInterval(() => secs++, 1000)
    try {
      cached = !!(await tools.check(id, fresh)).cached
      phase = 'ok'
      // The click on "Allow and enable" already said yes: a passing check turns it on.
      if (auto && !tools.get(id)?.enabled) await enable()
    } catch (e) {
      phase = 'failed'
      reason = explain(e instanceof CommandError ? e.code : '', e instanceof Error ? e.message : 'The check did not finish. Try again.')
    } finally {
      clearInterval(t)
      if (fresh) {
        await tick()
        box?.querySelector<HTMLElement>('[data-again]')?.focus()
      }
    }
  }

  onMount(() => void run())
  const again = () => void run(true)

  async function enable() {
    busy = true
    error = ''
    try {
      await tools.enable(id, true)
      if (!auto) onDone()
    } catch (e) {
      error = explain(e instanceof CommandError ? e.code : '', e instanceof Error ? e.message : 'Could not enable it. Try again.')
    } finally {
      busy = false
    }
  }
</script>

<div class="check" data-first tabindex="-1" bind:this={box}>
  {#if phase === 'running'}
    <p role="status">Checking {s?.name ?? 'the tool server'}… <span class="secs" aria-hidden="true">{secs} s</span></p>
    {#if s && isRemote(s.kind)}
      <p class="help">Mooring asks the server for its tools. This stops after 30 seconds.</p>
    {:else}
      <p class="help">Mooring starts it, asks for its tools, and stops it. The first run of an npx, uvx or docker server downloads it and can take up to 2 minutes. Later checks stop after 30 seconds.</p>
    {/if}
  {:else if phase === 'ok' && s}
    <p role="status">{s.name} answered with {s.tools.length} {s.tools.length === 1 ? 'tool' : 'tools'}.</p>
    {#if s.tools.length > 0}
      <ul>
        {#each s.tools as t (t.name)}<li><code>{t.name}</code>{#if t.description} <span class="muted">{t.description}</span>{/if}</li>{/each}
      </ul>
    {/if}
    <p class="help">{#if cached}Tools listed at {listedTime(s.checkedAt)} (saved). Check again for a fresh list.{:else}Tools listed at {listedTime(s.checkedAt)}.{/if}</p>
    <p class="help">{#if auto && s.enabled}It is allowed and enabled. {/if}Enabled servers can be picked when you start a chat. Nothing else uses it.</p>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
    <div class="acts">
      <button type="button" class="btn" data-again onclick={again}>Check again</button>
      {#if auto && s.enabled}
        <button type="button" class="btn primary" onclick={onDone}>Done</button>
      {:else}
        <button type="button" class="btn" onclick={onDone}>Not now</button>
        <button type="button" class="btn primary" disabled={busy || s.enabled} onclick={enable}>{s.enabled ? 'Enabled' : busy ? 'Enabling…' : 'Enable'}</button>
      {/if}
    </div>
  {:else if s && isRemote(s.kind) && s.signIn === 'needed'}
    <p class="error" role="alert">The check failed. {s.note || explain('SIGN_IN_REQUIRED', '')}</p>
    <div class="acts">
      <button type="button" class="btn" onclick={onDone}>Back to list</button>
      <button type="button" class="btn" data-again onclick={again}>Check again</button>
      <button type="button" class="btn primary" onclick={() => onSignIn(id)}>Sign in</button>
    </div>
  {:else}
    <p class="error" role="alert">The check failed.{#if auto} {s?.name ?? 'The server'} is saved and allowed, but not enabled.{/if}</p>
    <pre>{reason}</pre>
    <div class="acts">
      <button type="button" class="btn" onclick={onDone}>Back to list</button>
      <button type="button" class="btn primary" data-again onclick={again}>Check again</button>
    </div>
  {/if}
</div>

<style>
  .check {
    display: grid;
    gap: var(--s-3);
  }
  p {
    margin: 0;
  }
  ul {
    display: grid;
    gap: var(--s-1);
    margin: 0;
    padding: 0 0 0 var(--s-4);
  }
  code,
  pre {
    overflow-wrap: anywhere;
  }
  pre {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: var(--r-m);
    font-family: var(--font-mono);
    font-size: var(--fs-s);
    white-space: pre-wrap;
  }
  .muted {
    color: var(--muted);
  }
  /* Svelte drops the space at the start of the {#if}, so the description gets its own line under the name. */
  li .muted {
    display: block;
  }
  .acts {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: var(--s-2);
  }
</style>
