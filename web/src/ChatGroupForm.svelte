<script lang="ts">
  // In the chat menu: put this chat in a group with 1 or 2 other chats of the project, or open its group.
  import { actions } from './lib/actions'
  import { chats, type ChatInfo } from './lib/chatstore.svelte'
  import { groupCandidates, makeGroup, MAX_GROUP } from './lib/chatgroups'
  import { app } from './lib/state.svelte'

  let { id, info, done }: { id: string; info: ChatInfo; done: () => void } = $props()

  let open = $state(false)
  let picked = $state<string[]>([])
  let title = $state('')
  let error = $state('')
  let busy = $state(false)
  const uid = $props.id()
  const others = $derived(groupCandidates(chats.list, info))
  const full = $derived(picked.length >= MAX_GROUP - 1)

  function toggle(other: string, on: boolean) {
    picked = on ? [...picked, other] : picked.filter((x) => x !== other)
  }
  async function create() {
    error = ''
    busy = true
    try {
      const gid = await makeGroup((n, a) => app.run(n, a), [id, ...picked], title)
      done()
      actions.openGroup(gid)
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not make the group.'
    } finally {
      busy = false
    }
  }
</script>

{#if info.groupId}
  <button type="button" class="btn" onclick={() => (done(), actions.openGroup(info.groupId!))}>Open the group</button>
{:else if !open}
  <button type="button" class="btn" onclick={() => (open = true)}>Group with other chats…</button>
{:else}
  <fieldset>
    <legend tabindex="-1" {@attach (el: HTMLElement) => el.focus()}>Group with other chats</legend>
    {#if others.length === 0}
      <p class="help">No other chat in this project is free. Start another chat first. A chat can be in one group.</p>
    {:else}
      <p class="help">Pick 1 or 2 chats. They open next to this one, and each reply has "Send to…" to pass work on.</p>
      {#each others as o (o.id)}
        <label class="row"><input type="checkbox" checked={picked.includes(o.id)} disabled={full && !picked.includes(o.id)} onchange={(e) => toggle(o.id, e.currentTarget.checked)} />{o.title || 'New chat'}</label>
      {/each}
      <label class="field" for="{uid}-t">Group title (optional)</label>
      <input id="{uid}-t" class="input" maxlength="120" bind:value={title} />
      <button type="button" class="btn primary" disabled={busy || picked.length === 0} onclick={create}>Group chats</button>
    {/if}
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </fieldset>
{/if}

<style>
  fieldset {
    display: grid;
    gap: var(--s-2);
    margin: 0;
    padding: 0;
    border: 0;
    min-width: 0;
  }
  legend {
    padding: 0;
    font-weight: 600;
  }
  .help,
  .error {
    margin: 0;
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-height: var(--control-h);
  }
  .field {
    font-size: var(--fs-s);
    color: var(--muted);
  }
</style>
