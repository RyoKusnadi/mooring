<script lang="ts">
  import { checkSession } from './lib/api'
  import Login from './Login.svelte'
  import Shell from './Shell.svelte'
  import { watchAttention } from './lib/notifywatch.svelte'
  import { watchSeen } from './lib/seenwatch.svelte'

  type View = 'loading' | 'signed-in' | 'signed-out' | 'unreachable'
  let view = $state<View>('loading')

  // System notifications and the bell switch watch the stores while signed in.
  $effect(() => (view === 'signed-in' ? watchAttention() : undefined))
  // The server hears which chats are on screen, so "finished since you looked" stays true.
  $effect(() => (view === 'signed-in' ? watchSeen() : undefined))

  checkSession().then(
    (s) => (view = s),
    () => (view = 'unreachable'),
  )
</script>

{#if view === 'signed-in'}
  <Shell onSignOut={() => (view = 'signed-out')} />
{:else if view === 'signed-out'}
  <Login onSignIn={() => (view = 'signed-in')} />
{:else if view === 'unreachable'}
  <main class="notice">
    <p role="alert">Mooring's server is not answering. Check that it is running, then reload this page.</p>
  </main>
{:else}
  <main class="notice">
    <p class="loading" role="status">Checking your session…</p>
  </main>
{/if}

<style>
  .notice {
    display: grid;
    min-height: 100dvh;
    place-items: center;
    padding-top: env(safe-area-inset-top, 0px);
    padding-left: max(var(--s-4), env(safe-area-inset-left, 0px));
    padding-right: max(var(--s-4), env(safe-area-inset-right, 0px));
  }
  p {
    max-width: 40ch;
    margin: 0;
    padding: var(--s-3) var(--s-4);
    background: var(--surface);
    border: 1px solid var(--danger);
    border-radius: var(--r-m);
    color: var(--danger);
  }
  p.loading {
    padding: 0;
    background: none;
    border: 0;
    color: var(--muted);
  }
</style>
