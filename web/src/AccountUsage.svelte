<script lang="ts">
  // Usage of one account: each window the agent reported, and the login state.
  import type { Account } from './lib/accountapi'
  import { limited, loginLabel, percentText, resetText, usageKnown, windowName } from './lib/accountfmt'

  let { account: a }: { account: Account } = $props()
  const hit = $derived(limited(a.usage))
</script>

<div class="usage">
  <span class="login" class:att={a.login === 'needed'}>{loginLabel[a.login] ?? loginLabel.unknown}</span>
  {#if hit}
    <span class="login att">Limit reached, {resetText(a.usage.limitResetsAt)}</span>
  {/if}
  {#if usageKnown(a.usage)}
    <ul aria-label="Usage of {a.name}">
      {#each a.usage.windows ?? [] as w (w.name)}
        <li>
          <span class="w">{windowName(w.name)}</span>
          <span>{percentText(w)} used</span>
          <span class="muted">{resetText(w.resetsAt)}</span>
        </li>
      {/each}
    </ul>
  {:else}
    <span class="muted">Usage: unknown. {a.usage.source === 'unknown' ? 'This agent has not reported any yet.' : ''}</span>
  {/if}
</div>

<style>
  .usage {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--s-1);
    font-size: var(--fs-s);
  }
  .login {
    font-weight: 600;
  }
  .att {
    padding: 0 var(--s-1);
    border-radius: var(--r-s);
    background: var(--attention-bg);
    color: var(--attention);
  }
  .muted {
    color: var(--muted);
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
    flex-wrap: wrap;
    gap: 0 var(--s-2);
  }
  .w {
    min-width: 7em;
    font-weight: 600;
  }
</style>
