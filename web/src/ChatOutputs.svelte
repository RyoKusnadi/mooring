<script lang="ts">
  // The raw output of commands the agent ran through Mooring: how much of it the agent got, a switch
  // to read all of it here, and a button to send all of it with the next message.
  import { attach } from './lib/attachstore.svelte'
  import { fetchOutput } from './lib/settingsapi'
  import { outputCount, type OutputRef } from './lib/tokenfmt'

  let { chatId, outputs }: { chatId: string; outputs: OutputRef[] } = $props()

  let shown = $state<Record<string, string | null>>({})
  let problem = $state<Record<string, string>>({})
  let sent = $state<Record<string, boolean>>({})

  async function toggle(o: OutputRef) {
    problem[o.id] = ''
    if (shown[o.id] != null) return void (shown[o.id] = null)
    try {
      shown[o.id] = await fetchOutput(chatId, o.id)
    } catch (e) {
      problem[o.id] = e instanceof Error ? e.message : 'Could not read the output.'
    }
  }

  function send(o: OutputRef) {
    const err = attach.add(chatId, { kind: 'output', outputId: o.id, lines: o.lines })
    problem[o.id] = err
    sent[o.id] = !err
  }
</script>

{#each outputs as o (o.id)}
  <div class="out">
    <span class="count">{outputCount(o)}</span>
    <label class="sw"><input type="checkbox" role="switch" checked={shown[o.id] != null} onchange={() => toggle(o)} /> Show raw output</label>
    <button type="button" class="btn small" disabled={sent[o.id]} onclick={() => send(o)}>{sent[o.id] ? 'Added to the next message' : 'Send full output'}</button>
    {#if problem[o.id]}<span class="error" role="alert">{problem[o.id]}</span>{/if}
    {#if shown[o.id] != null}<pre>{shown[o.id]}</pre>{/if}
  </div>
{/each}

<style>
  .out {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-1) var(--s-3);
    padding: var(--s-1) var(--s-2);
    border-top: 1px solid var(--border);
    font-size: var(--fs-s);
  }
  .count {
    color: var(--muted);
  }
  .sw {
    display: inline-flex;
    gap: var(--s-1);
    align-items: center;
  }
  .small {
    min-height: 1.75rem;
    padding-block: 0;
  }
  @media (pointer: coarse) {
    .sw,
    .small {
      min-height: var(--control-h);
    }
  }
  pre {
    flex-basis: 100%;
    max-height: 24rem;
    margin: 0;
    padding: var(--s-2);
    overflow: auto;
    background: var(--bg);
    font-family: var(--font-mono);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
