<script lang="ts">
  // Notifications section of Settings: system notifications for chats and terminals that need you,
  // and whether a terminal bell counts. Both are kept in this browser only.
  import { tick } from 'svelte'
  import { askPermission, bellCounts, permission, setBellCounts, type Permission } from './lib/notify'

  let perm = $state<Permission>(permission())
  let bell = $state(bellCounts())
  let result: HTMLElement | undefined = $state()
  let button: HTMLButtonElement | undefined = $state()
  const phone = matchMedia('(pointer: coarse)').matches

  // The button goes away once the browser answers, so focus moves to the answer (or back to the button).
  async function allow() {
    perm = await askPermission()
    await tick()
    ;(result ?? button)?.focus()
  }
</script>

<p class="help lead">
  Show a system notification when a chat or terminal needs you and this tab is in the background. It names the project only, never what the agent wrote.
</p>

{#if perm === 'granted'}
  <p class="status" role="status" tabindex="-1" bind:this={result}>Notifications are on in this browser.</p>
{:else if perm === 'denied'}
  <p class="help blocked" role="status" tabindex="-1" bind:this={result}>This browser blocks notifications for Mooring. Allow them in the site settings next to the address, then come back.</p>
{:else if perm === 'unsupported'}
  <p class="help" role="status">This browser has no system notifications.</p>
{:else}
  <button type="button" class="btn" bind:this={button} onclick={allow}>Allow notifications</button>
{/if}
{#if phone}<p class="help">Phones get no notifications while Mooring is closed.</p>{/if}

<label class="row">
  <input
    type="checkbox"
    checked={bell}
    onchange={(e) => {
      bell = e.currentTarget.checked
      setBellCounts(bell)
    }}
  />
  <span>
    <strong>A terminal bell counts as needing you</strong>
    <span class="help">Some shells ring the bell on tab completion. Turn this off if terminals get marked too often.</span>
  </span>
</label>

<style>
  .lead {
    margin: 0 0 var(--s-3);
  }
  .status {
    margin: 0 0 var(--s-3);
    color: var(--success);
  }
  .blocked {
    margin: 0 0 var(--s-3);
    color: var(--muted);
  }
  p[tabindex='-1']:focus-visible {
    outline-offset: 2px;
  }
  .btn {
    margin-bottom: var(--s-3);
  }
  .row {
    display: flex;
    gap: var(--s-3);
    align-items: flex-start;
    margin-top: var(--s-4);
  }
  .row input {
    margin-top: var(--s-1);
    flex: none;
  }
  .row span {
    display: grid;
    gap: var(--s-1);
  }
</style>
